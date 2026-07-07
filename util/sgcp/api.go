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

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	b, err = io.ReadAll(resp.Body)

	return resp.StatusCode, b, err
}
