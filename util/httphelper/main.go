// Package httphelper implements NewRequest, Do, DoRequest from
// a requestfactory.T and http.client
package httphelper

import (
	"crypto/tls"
	"fmt"
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
	if t == nil {
		return nil, fmt.Errorf("do from undef httphelper")
	}
	if t.client == nil {
		return nil, fmt.Errorf("do from undef httphelper client")
	}
	return t.client.Do(req)
}

func (t *T) DoRequest(method string, relPath string, body io.Reader) (*http.Response, error) {
	if t == nil {
		return nil, fmt.Errorf("do request from undef httphelper")
	}
	if t.client == nil {
		return nil, fmt.Errorf("do request from undef httphelper client")
	}
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
	if t == nil {
		return nil, fmt.Errorf("new request from undef httphelper")
	}
	if t.factory == nil {
		return nil, fmt.Errorf("new request from undef request factory")
	}
	return t.factory.NewRequest(method, relPath, body)
}
