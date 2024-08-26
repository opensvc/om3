package uri

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/file"
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

func (t T) Fetch() (string, error) {
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
	fName := f.Name()
	if _, err = io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return fName, nil
}

func (t T) IsValid() bool {
	return IsValid(t.uri)
}

func IsValid(s string) bool {
	_, err := url.ParseRequestURI(s)
	if err != nil {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	return true
}

func ReadAllFrom(from string) ([]byte, error) {
	switch from {
	case "":
		return nil, ErrFromEmpty
	case "-", "stdin", "/dev/stdin":
		return readAllFromStdin()
	default:
		u := New(from)
		if u.IsValid() {
			return readAllFromURI(u)
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

func readAllFromStdin() ([]byte, error) {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		reader := bufio.NewReader(os.Stdin)
		return io.ReadAll(reader)
	}
	return nil, fmt.Errorf("stdin must be a pipe")
}

func readAllFromRegular(p string) ([]byte, error) {
	return os.ReadFile(p)
}

func readAllFromDir(p string) ([]byte, error) {
	return nil, fmt.Errorf("TODO")
}

func readAllFromURI(u T) ([]byte, error) {
	fName, err := u.Fetch()
	if err != nil {
		return nil, err
	}
	defer os.Remove(fName)
	return readAllFromRegular(fName)
}
