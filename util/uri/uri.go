package uri

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/httppolicy"
	"github.com/opensvc/om3/v3/util/random"
)

type (
	T struct {
		uri string
	}
)

var (
	ErrFromUnknown = errors.New("from is unknown")
	ErrFromEmpty   = errors.New("from is empty")
)

func New(s string) T {
	return T{
		uri: s,
	}
}

func CheckHttpUrl(s string) (net.IP, string, error) {
	return checkHttpUrl(s)
}

// IsValidHttp validates if the provided string is a well-formed HTTP or HTTPS URL.
func IsValidHttp(s string) bool {
	_, err := url.ParseRequestURI(s)
	if err != nil {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "http", "https":
		return true
	default:
		return false
	}
}

// SafeHttpClient returns an HTTP client configured with rules to prevent
// SSRF vulnerabilities for the provided URL.
func SafeHttpClient(s string) (*http.Client, error) {
	return safeHttpClient(s)
}

// IsValidHttp checks if the encapsulated URI is a valid HTTP or HTTPS URL.
func (t T) IsValidHttp() bool {
	return IsValidHttp(t.uri)
}

func (t T) HttpFetch() (string, error) {
	var resp *http.Response

	client, err := SafeHttpClient(t.uri)
	if err != nil {
		return "", err
	}

	resp, err = client.Get(t.uri)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("fetch %s error %d: %s", t.uri, resp.StatusCode, resp.Status)
	}
	createTemp := func() (*os.File, error) {
		return os.CreateTemp(rawconfig.Paths.Tmp, ".fetch.*")
	}
	f, err := createTemp()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err := os.MkdirAll(rawconfig.Paths.Tmp, os.ModePerm); err != nil {
			return "", err
		}
		if f, err = createTemp(); err != nil {
			return "", err
		}
	}
	fName := f.Name()
	if _, err = io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return fName, nil
}

func ReadAllFrom(from string) (map[string][]byte, error) {
	switch from {
	case "":
		return nil, ErrFromEmpty
	case "-", "stdin", "/dev/stdin":
		return readAllFromStdin()
	case "/dev/urandom":
		return readFromURandom()
	default:
		u := New(from)
		if u.IsValidHttp() {
			return readAllFromHttp(u)
		}
		if v, err := file.ExistsAndRegular(from); err != nil {
			return nil, err
		} else if v {
			return readAllFromRegular(from)
		}
		if v, err := file.ExistsAndDir(from); err != nil {
			return nil, err
		} else if v {
			return readAllFromDir(from)
		}
		return nil, ErrFromUnknown
	}
}

func clientForIP(ip net.IP, port string, enableRedirects bool) *http.Client {
	dialer := &net.Dialer{}

	transport := &http.Transport{
		DialContext: func(
			ctx context.Context,
			network, _ string,
		) (net.Conn, error) {
			return dialer.DialContext(
				ctx,
				network,
				net.JoinHostPort(ip.String(), port),
			)
		},
	}

	client := &http.Client{
		Transport: transport,
	}

	if !enableRedirects {
		// Never follow redirects.
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client
}

func checkHttpUrl(s string) (net.IP, string, error) {
	policy := httppolicy.New(rawconfig.SSRFAllowedURL, rawconfig.SSRFBlockedURL, rawconfig.SSRFAllowedCIDR, rawconfig.SSRFBlockedCIDR)
	return policy.Check(s)
}

func safeHttpClient(s string) (*http.Client, error) {
	ip, port, err := checkHttpUrl(s)
	if err != nil {
		return nil, err
	}
	return clientForIP(ip, port, rawconfig.SSRFEnableRedirects), nil
}

func readAllFromStdin() (map[string][]byte, error) {
	m := make(map[string][]byte)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		reader := bufio.NewReader(os.Stdin)
		b, err := io.ReadAll(reader)
		m[""] = b
		return m, err
	}
	return m, fmt.Errorf("stdin must be a pipe")
}

func readFromURandom() (map[string][]byte, error) {
	m := make(map[string][]byte)
	m[""] = random.Password(32, nil)
	return m, nil
}

func readAllFromRegular(p string) (map[string][]byte, error) {
	m := make(map[string][]byte)
	b, err := os.ReadFile(p)
	m[""] = b
	return m, err
}

func readAllFromDir(p string) (map[string][]byte, error) {
	m := make(map[string][]byte)
	err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err != nil {
				return err
			}
			m[path] = append([]byte{}, b...)
		}
		return nil
	})
	return m, err
}

func readAllFromHttp(u T) (map[string][]byte, error) {
	fName, err := u.HttpFetch()
	if err != nil {
		return nil, err
	}
	defer os.Remove(fName)
	return readAllFromRegular(fName)
}
