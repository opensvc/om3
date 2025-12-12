package uri

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/file"
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

func ReadAllFrom(from string) (map[string][]byte, error) {
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

func readAllFromURI(u T) (map[string][]byte, error) {
	fName, err := u.Fetch()
	if err != nil {
		return nil, err
	}
	defer os.Remove(fName)
	return readAllFromRegular(fName)
}
