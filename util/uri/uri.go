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
	"slices"
	"sync"

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

const (
	// FromRandom is the from generating a random password, as the
	// "source random" of an install line or an environment reference.
	FromRandom = "random"

	// fromURandom is the from "key add" took for a generator before
	// FromRandom. It still generates, deprecated: everywhere else a source
	// naming /dev/urandom is the path it names.
	fromURandom = "/dev/urandom"
)

// DeprecatedFrom says a from is deprecated, and returns the one to use
// instead, for the command to warn of: /dev/urandom, which reads as a path
// and generates a password, is FromRandom.
func DeprecatedFrom(from string) (string, bool) {
	if from == fromURandom {
		return FromRandom, true
	}
	return "", false
}

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

// CallerFetch fetches the url a command is given, in the process of the
// command, and returns the path of a temporary file holding the body.
//
// It is held to no SSRF policy: the url is the argument of who runs the
// command, fetched from where they run it, with nothing more than they
// could fetch with curl and pass on the standard input of the same command.
// A url a configuration names, fetched by a node, as the source of a key or
// the image of a vm, is the SafeHttpClient's to fetch.
func (t T) CallerFetch() (string, error) {
	resp, err := http.Get(t.uri)
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
	defer f.Close()
	fName := f.Name()
	if _, err = io.Copy(f, resp.Body); err != nil {
		_ = os.Remove(fName)
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
	case FromRandom, fromURandom:
		return readFromRandom()
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

// maxRedirects is how many redirects a fetch follows.
const maxRedirects = 5

// pinnedClient returns a client dialing only the addresses the policy
// accepted: the one of the url fetched, and the one of each redirect target,
// checked as it comes. A host resolved again at dial time could answer with
// another address than the one checked.
//
// A redirect is followed when enabled, by a GET or a HEAD only, at most
// maxRedirects times, never from https to http, its target held to the cidr lists and not to the url
// lists: the url lists judge the url a configuration names, and where that
// source keeps its content, as the storage a release download redirects to,
// is the source's to say.
func pinnedClient(policy *httppolicy.T, rawURL string, ip net.IP, port string, enableRedirects bool) (*http.Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	pinned := map[string]string{
		net.JoinHostPort(u.Hostname(), port): net.JoinHostPort(ip.String(), port),
	}
	dialer := &net.Dialer{}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			mu.Lock()
			target, ok := pinned[addr]
			mu.Unlock()
			if !ok {
				return nil, fmt.Errorf("dial %s: not an address the policy accepted", addr)
			}
			return dialer.DialContext(ctx, network, target)
		},
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !enableRedirects {
				return http.ErrUseLastResponse
			}
			// A download follows a redirect. A request carrying data, as
			// the calls of an array api, does not: a 307 or 308 would send
			// its body again, to whatever host the redirect names.
			if m := via[0].Method; m != http.MethodGet && m != http.MethodHead {
				return fmt.Errorf("redirect of a %s request refused: only a download follows a redirect", m)
			}
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect from https to %s refused", req.URL.Scheme)
			}
			ip, port, err := policy.CheckRedirect(req.URL.String())
			if err != nil {
				return fmt.Errorf("redirect to %s: %w", req.URL.Redacted(), err)
			}
			mu.Lock()
			pinned[net.JoinHostPort(req.URL.Hostname(), port)] = net.JoinHostPort(ip.String(), port)
			mu.Unlock()
			return nil
		},
	}
	return client, nil
}

// ClusterSources returns the urls and the address ranges the cluster allows on
// top of the policy of the node, which the object layer sets: a util knows
// nothing of the configuration of a cluster. Left unset, as in a client with no
// cluster, the policy of the node is the whole of it.
var ClusterSources func() (urls, cidrs []string)

// ssrfPolicy returns the policy of this node, with the sources the cluster
// allows on top.
func ssrfPolicy() *httppolicy.T {
	urls, cidrs := rawconfig.SSRFAllowedURL, rawconfig.SSRFAllowedCIDR
	if ClusterSources != nil {
		moreURLs, moreCIDRs := ClusterSources()
		urls = append(slices.Clone(urls), moreURLs...)
		cidrs = append(slices.Clone(cidrs), moreCIDRs...)
	}
	return httppolicy.New(urls, rawconfig.SSRFBlockedURL, cidrs, rawconfig.SSRFBlockedCIDR)
}

func checkHttpUrl(s string) (net.IP, string, error) {
	return ssrfPolicy().Check(s)
}

func safeHttpClient(s string) (*http.Client, error) {
	policy := ssrfPolicy()
	ip, port, err := policy.Check(s)
	if err != nil {
		return nil, err
	}
	return pinnedClient(policy, s, ip, port, rawconfig.SSRFEnableRedirects)
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

func readFromRandom() (map[string][]byte, error) {
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
	fName, err := u.CallerFetch()
	if err != nil {
		return nil, err
	}
	defer os.Remove(fName)
	return readAllFromRegular(fName)
}
