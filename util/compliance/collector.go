package compliance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// collectorTimeout bounds a request to the collector.
const collectorTimeout = time.Minute

// collectorDo sends a request to the compliance routes of the collector, and
// decodes its answer in out, if set.
func (t T) collectorDo(method, path string, query url.Values, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		body = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), collectorTimeout)
	defer cancel()
	req, err := t.collectorClient.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return fmt.Errorf("create collector request %s %s: %w", method, path, err)
	}
	req.URL.RawQuery = query.Encode()
	resp, err := t.collectorClient.Do(req)
	if err != nil {
		return fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("collector %s %s: %s", method, path, collectorProblem(resp))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode collector response to %s %s: %w", method, path, err)
	}
	return nil
}

func collectorProblem(resp *http.Response) string {
	var problem struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&problem); err != nil || problem.Text == "" {
		return fmt.Sprintf("unexpected response status code %d", resp.StatusCode)
	}
	return fmt.Sprintf("[%d] %s", resp.StatusCode, problem.Text)
}

// objectQuery returns the query parameters selecting the object of this
// compliance, none for the node.
func (t T) objectQuery() url.Values {
	query := url.Values{}
	if !t.objectPath.IsZero() {
		query.Set("path", t.objectPath.String())
	}
	return query
}

// attachment attaches or detaches a moduleset or a ruleset, of the node or of
// the object of this compliance.
func (t T) attachment(method, kind, name string) error {
	var info struct {
		Info string `json:"info"`
	}
	path := fmt.Sprintf("%s/%ss/%s", collectorAgentPath, kind, url.PathEscape(strings.TrimSpace(name)))
	if err := t.collectorDo(method, path, t.objectQuery(), nil, &info); err != nil {
		return err
	}
	t.log.Infof(info.Info)
	return nil
}

func (t T) listNames(kind, filter string) ([]string, error) {
	data := make([]string, 0)
	if filter == "" {
		filter = "%"
	}
	query := url.Values{"pattern": []string{filter}}
	if err := t.collectorDo(http.MethodGet, collectorAgentPath+"/"+kind+"s", query, nil, &data); err != nil {
		return nil, err
	}
	return data, nil
}
