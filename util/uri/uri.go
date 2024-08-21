package uri

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/opensvc/om3/core/rawconfig"
)

type (
	T struct {
		uri string
	}
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
