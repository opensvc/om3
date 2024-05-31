// Package httphelper implements NewRequest, Do, DoRequest from
// a requestfactory.T and http.client
package httphelper

import (
	"crypto/tls"
	"io"
	"net/http"

	"github.com/opensvc/om3/util/requestfactory"
)

type (
	T struct {
		client  *http.Client
		factory *requestfactory.T
	}
)

func (t *T) Do(req *http.Request) (*http.Response, error) {
	return t.client.Do(req)
}

func (t *T) DoRequest(method string, relPath string, body io.Reader) (*http.Response, error) {
	if req, err := t.NewRequest(method, relPath, body); err != nil {
		return nil, err
	} else {
		return t.client.Do(req)
	}
}

func New(cli *http.Client, factory *requestfactory.T) *T {
	return &T{
		client:  cli,
		factory: factory,
	}
}

func NewHttpsClient(insecure bool) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecure,
			},
		},
	}
}

func (t *T) NewRequest(method string, relPath string, body io.Reader) (*http.Request, error) {
	return t.factory.NewRequest(method, relPath, body)
}
