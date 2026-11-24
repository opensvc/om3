package object

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/opensvc/om3/v3/core/oc3path"
)

// wakeCollectorAnswer is the answer of the collector to a wake: the start of
// the container running the node, queued on its hypervisor.
type wakeCollectorAnswer struct {
	ActionID   int64  `json:"action_id"`
	Hypervisor string `json:"hypervisor"`
	Nodename   string `json:"nodename"`
	Path       string `json:"path"`
	RID        string `json:"rid"`
	Queued     bool   `json:"queued"`
}

// wakeByCollector is the wake through the collector, replaced in tests.
var wakeByCollector = (*Node).wakeByCollector

// wakeByCollector asks the collector to wake the virtual machine node
// nodename, of the cluster of this node: the collector queues the start of
// the container resource of its hypervisor whose virtual machine has the
// machine uuid the node reported. The hypervisor must report to the same
// collector.
func (t *Node) wakeByCollector(nodename string) (wakeCollectorAnswer, error) {
	var (
		ans    wakeCollectorAnswer
		method = http.MethodPost
		path   = oc3path.ServerAgentWake
	)
	oc3, err := t.CollectorServer()
	if err != nil {
		return ans, err
	}
	ctx, cancel, timeout := t.collectorRequestContext(context.Background())
	defer cancel()
	req, err := oc3.NewRequestWithContext(ctx, method, path, nil)
	if err != nil {
		return ans, fmt.Errorf("create collector request %s %s: %w", method, path, err)
	}
	// The query goes on the request: the request factory joins the path,
	// and would escape a ? written in it.
	req.URL.RawQuery = url.Values{"nodename": {nodename}}.Encode()
	resp, err := oc3.Do(req)
	if err != nil {
		return ans, collectorRequestError(err, method, path, timeout)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := CollectorResponseStatusCheck(resp, method, path, []int{http.StatusOK}); err != nil {
		return ans, err
	}
	if err := json.NewDecoder(resp.Body).Decode(&ans); err != nil {
		return ans, fmt.Errorf("decode response body: %w", err)
	}
	return ans, nil
}
