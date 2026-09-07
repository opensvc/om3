package sgcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	Api struct {
		client *http.Client
		tk     tokenGetter
		log    *plog.Logger
	}

	tokenGetter interface {
		Get(ctx context.Context, scope ...string) (string, error)
	}
)

func (a *Api) CheckStatusCode(method, url string, got int, wanted ...int) error {
	a.log.Debugf("%s %s status code: %d", method, url, got)
	if slices.Contains(wanted, got) {
		return nil
	}
	return fmt.Errorf("unexpected status code for %s %s got %d wanted %v", method, url, got, wanted)
}

// do executes the request and reports the response status code and body.
//
// The status code is an answer, not a failure: a 404 saying the filesystem is
// gone, or a 412 saying the consistency group is busy, is information the
// callers act on. So err is reserved for what prevents an answer altogether
// (token, request build, transport, body read), and each caller declares the
// status codes it accepts, with CheckStatusCode.
func (a *Api) do(ctx context.Context, method, url string, body io.Reader, scopes ...string) (statusCode int, b []byte, err error) {
	var req *http.Request
	var resp *http.Response

	bearer, err := a.tk.Get(ctx, scopes...)
	if err != nil {
		return 0, nil, err
	}

	req, err = http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return 0, nil, err
	}

	// Add headers for authentication
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearer))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	a.log.Debugf("request: %s %s", method, url)
	resp, err = a.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	a.log.Debugf("request: %s %s status code: %d", method, url, resp.StatusCode)

	b, err = io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read %s %s response body: %w", method, url, err)
	}
	if resp.StatusCode >= 400 {
		a.log.Debugf("request: %s %s status code: %d body: '%s'", method, url, resp.StatusCode, string(b))
	}

	return resp.StatusCode, b, nil
}
