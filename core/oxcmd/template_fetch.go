package oxcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/tmplrender"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	// templateFetcher fetches from the api the values a template reads no
	// event carries.
	templateFetcher struct {
		c *client.T

		// debugf logs the api calls, never the values they return,
		// when not nil.
		debugf func(string, ...any)
	}

	statusCoder interface {
		StatusCode() int
		Status() string
	}
)

// logCall logs an api call begun at begin, with its response, or err.
func (t *templateFetcher) logCall(begin time.Time, resp *http.Response, err error) {
	if t.debugf == nil {
		return
	}
	d := time.Since(begin).Round(time.Millisecond)
	switch {
	case err != nil:
		t.debugf("api: %s", err)
	case resp == nil || resp.Request == nil:
	default:
		t.debugf("api: %s %s: %s in %s", resp.Request.Method, resp.Request.URL.RequestURI(), resp.Status, d)
	}
}

// apiError returns the error of a response neither 200 nor 404, with the
// detail the api gives, never the value asked.
func apiError(what string, resp statusCoder, body []byte) error {
	var problem struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	detail := resp.Status()
	if err := json.Unmarshal(body, &problem); err == nil && (problem.Title != "" || problem.Detail != "") {
		detail = strings.TrimSpace(problem.Title + ": " + problem.Detail)
	}
	if resp.StatusCode() == http.StatusForbidden {
		detail += " (reading a sec key needs the secreader or the admin role on its namespace)"
	}
	return fmt.Errorf("%s: %s", what, detail)
}

func (t *templateFetcher) DataKey(ctx context.Context, p naming.Path, name string) ([]byte, error) {
	begin := time.Now()
	resp, err := t.c.GetObjectDataKeyWithResponse(ctx, p.Namespace, p.Kind, p.Name, &api.GetObjectDataKeyParams{Name: name})
	t.logCall(begin, httpResponse(resp), err)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		return resp.Body, nil
	case http.StatusNotFound:
		return nil, tmplrender.ErrNotFound
	default:
		return nil, apiError(fmt.Sprintf("key %s of %s", name, p), resp, resp.Body)
	}
}

func (t *templateFetcher) DataKeyNames(ctx context.Context, p naming.Path) ([]string, error) {
	begin := time.Now()
	resp, err := t.c.GetObjectDataKeysWithResponse(ctx, p.Namespace, p.Kind, p.Name, &api.GetObjectDataKeysParams{})
	t.logCall(begin, httpResponse(resp), err)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		l := make([]string, 0, len(resp.JSON200.Items))
		for _, item := range resp.JSON200.Items {
			l = append(l, item.Name)
		}
		return l, nil
	case http.StatusNotFound:
		return nil, tmplrender.ErrNotFound
	default:
		return nil, apiError(fmt.Sprintf("keys of %s", p), resp, resp.Body)
	}
}

func (t *templateFetcher) ConfigValue(ctx context.Context, p naming.Path, kw, node string) (string, error) {
	evaluate := true
	params := api.GetObjectConfigParams{Evaluate: &evaluate, Kw: &[]string{kw}}
	if node != "" {
		params.Impersonate = &node
	}
	begin := time.Now()
	resp, err := t.c.GetObjectConfigWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
	t.logCall(begin, httpResponse(resp), err)
	if err != nil {
		return "", err
	}
	what := fmt.Sprintf("keyword %s of %s", kw, p)
	switch resp.StatusCode() {
	case http.StatusOK:
		for _, item := range resp.JSON200.Items {
			if item.EvaluatedText != nil {
				return *item.EvaluatedText, nil
			}
			return item.Value, nil
		}
		return "", fmt.Errorf("%s: %w", what, tmplrender.ErrNotFound)
	case http.StatusNotFound:
		return "", fmt.Errorf("%s: %w", what, tmplrender.ErrNotFound)
	default:
		return "", apiError(what, resp, resp.Body)
	}
}

func (t *templateFetcher) ResourceInfo(ctx context.Context, p naming.Path, node, rid, key string) (string, error) {
	begin := time.Now()
	resp, err := t.c.GetObjectResourceInfoWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	t.logCall(begin, httpResponse(resp), err)
	if err != nil {
		return "", err
	}
	what := fmt.Sprintf("information %s of %s %s on %s", key, p, rid, node)
	switch resp.StatusCode() {
	case http.StatusOK:
		for _, item := range resp.JSON200.Items {
			if item.Node == node && item.Rid == rid && item.Key == key {
				return item.Value, nil
			}
		}
		return "", fmt.Errorf("%s: %w", what, tmplrender.ErrNotFound)
	case http.StatusNotFound:
		return "", fmt.Errorf("%s: %w", what, tmplrender.ErrNotFound)
	default:
		return "", apiError(what, resp, resp.Body)
	}
}

// httpResponse returns the http response of resp, nil when the call failed.
func httpResponse(resp interface{ StatusCode() int }) *http.Response {
	if resp == nil {
		return nil
	}
	switch r := resp.(type) {
	case *api.GetObjectDataKeyResponse:
		if r != nil {
			return r.HTTPResponse
		}
	case *api.GetObjectDataKeysResponse:
		if r != nil {
			return r.HTTPResponse
		}
	case *api.GetObjectConfigResponse:
		if r != nil {
			return r.HTTPResponse
		}
	case *api.GetObjectResourceInfoResponse:
		if r != nil {
			return r.HTTPResponse
		}
	}
	return nil
}
