package uri

import (
	"fmt"
	"io"
	"io/ioutil"
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
	var (
		f   *os.File
		err error
	)
	resp, err := http.Get(t.uri)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if f, err = ioutil.TempFile(rawconfig.Paths.Tmp, ".fetch.*"); err != nil {
		return "", err
	}
	fName := f.Name()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("fetch %s error %d: %s", t.uri, resp.StatusCode, resp.Status)
	}
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
