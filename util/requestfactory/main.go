// Package requestfactory provides *http.Request factory with default headers and base url.
package requestfactory

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

type (
	T struct {
		URL    *url.URL
		header http.Header
	}
)

// New returns http.Request factory for server with default headers.
func New(server *url.URL, header http.Header) *T {
	return &T{
		URL:    server,
		header: header,
	}
}

func (r *T) NewRequest(method, relPath string, body io.Reader) (req *http.Request, err error) {
	return r.NewRequestWithContext(context.Background(), method, relPath, body)
}

func (r *T) NewRequestWithContext(ctx context.Context, method, relPath string, body io.Reader) (req *http.Request, err error) {
	var url *url.URL
	url, err = r.URL.Parse(r.URL.JoinPath(relPath).String())
	if err != nil {
		return
	}

	req, err = http.NewRequestWithContext(ctx, method, url.String(), body)
	if err != nil {
		return
	}

	req.Header = r.header.Clone()
	return
}
