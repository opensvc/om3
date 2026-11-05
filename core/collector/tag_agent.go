package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/util/httphelper"
)

type (
	// TagClient calls the tag routes the collector serves to the nodes: they
	// act on the node authenticated, or on one of its objects.
	TagClient struct {
		cli *httphelper.T
	}

	tagCreate struct {
		Name    string  `json:"name"`
		Data    *string `json:"data,omitempty"`
		Exclude *string `json:"exclude,omitempty"`
	}

	tagAttach struct {
		AttachData *string `json:"attach_data,omitempty"`
	}

	tagInfo struct {
		Info string `json:"info"`
	}
)

// tagTimeout bounds a request to the collector.
const tagTimeout = time.Minute

// NewTagClient returns a TagClient sending its requests with cli, a client of
// the collector server authenticated as the node.
func NewTagClient(cli *httphelper.T) *TagClient {
	return &TagClient{cli: cli}
}

// Names returns the names of the tags.
func (t *TagClient) Names() ([]string, error) {
	l := make([]string, 0)
	if err := t.do(http.MethodGet, oc3path.ServerAgentTags, nil, nil, &l); err != nil {
		return nil, err
	}
	return l, nil
}

// Attached returns the tags attached to the object at p, or to the node if p
// is zero.
func (t *TagClient) Attached(p naming.Path) (TagAttachmentList, error) {
	l := make(TagAttachmentList, 0)
	if err := t.do(http.MethodGet, oc3path.ServerAgentTagsAttached, pathQuery(p), nil, &l); err != nil {
		return nil, err
	}
	return l, nil
}

// Create creates the tag name, and returns the answer of the collector.
func (t *TagClient) Create(name string, data, exclude *string) (string, error) {
	var info tagInfo
	body := tagCreate{Name: name, Data: data, Exclude: exclude}
	if err := t.do(http.MethodPost, oc3path.ServerAgentTags, nil, body, &info); err != nil {
		return "", err
	}
	return info.Info, nil
}

// Attach attaches the tag name to the object at p, or to the node if p is
// zero, and returns the answer of the collector.
func (t *TagClient) Attach(name string, p naming.Path, attachData *string) (string, error) {
	var info tagInfo
	if err := t.do(http.MethodPost, tagPath(name), pathQuery(p), tagAttach{AttachData: attachData}, &info); err != nil {
		return "", err
	}
	return info.Info, nil
}

// Detach detaches the tag name from the object at p, or from the node if p
// is zero, and returns the answer of the collector.
func (t *TagClient) Detach(name string, p naming.Path) (string, error) {
	var info tagInfo
	if err := t.do(http.MethodDelete, tagPath(name), pathQuery(p), nil, &info); err != nil {
		return "", err
	}
	return info.Info, nil
}

func tagPath(name string) string {
	return oc3path.ServerAgentTags + "/" + url.PathEscape(name)
}

func pathQuery(p naming.Path) url.Values {
	query := url.Values{}
	if !p.IsZero() {
		query.Set("path", p.String())
	}
	return query
}

func (t *TagClient) do(method, path string, query url.Values, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode the request body: %w", err)
		}
		body = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), tagTimeout)
	defer cancel()
	req, err := t.cli.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return fmt.Errorf("create the collector request %s %s: %w", method, path, err)
	}
	req.URL.RawQuery = query.Encode()
	resp, err := t.cli.Do(req)
	if err != nil {
		return fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("collector %s %s: %s", method, path, problem(resp))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode the collector answer to %s %s: %w", method, path, err)
	}
	return nil
}

// problem returns the explanation the collector gave of a refusal, or its
// status code if it gave none.
func problem(resp *http.Response) string {
	var p struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil || p.Text == "" {
		return fmt.Sprintf("unexpected status code %d", resp.StatusCode)
	}
	return fmt.Sprintf("[%d] %s", resp.StatusCode, p.Text)
}
