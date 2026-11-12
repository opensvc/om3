// Package httphelper implements NewRequest, Do, DoRequest from
// a requestfactory.T and http.client
package httphelper

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/opensvc/om3/v3/util/requestfactory"
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

func (t *T) URL() string {
	return t.factory.URL.String()
}

func New(cli *http.Client, factory *requestfactory.T) *T {
	return &T{
		client:  cli,
		factory: factory,
	}
}

// NewHttpsClient returns a client of the transport of the nodes, its connect
// and its TLS handshake bounded, verifying the certificate of the server
// unless insecure.
//
// The transport is shared by the clients: a request reuses the connection of
// the previous ones. A request with no connection by half the time its
// context leaves it, nothing sent, is sent again once, and a request failing
// says the phase it was in, as a PhaseError.
func NewHttpsClient(insecure bool) *http.Client {
	return &http.Client{
		Transport: &retryUnsent{base: sharedTransport(insecure)},
	}
}

func (t *T) NewRequest(method string, relPath string, body io.Reader) (*http.Request, error) {
	return t.NewRequestWithContext(context.Background(), method, relPath, body)
}

func (t *T) NewRequestWithContext(ctx context.Context, method string, relPath string, body io.Reader) (*http.Request, error) {
	if t == nil {
		return nil, fmt.Errorf("new request from undef httphelper")
	}
	if t.factory == nil {
		return nil, fmt.Errorf("new request from undef request factory")
	}
	return t.factory.NewRequestWithContext(ctx, method, relPath, body)
}
