package object

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/util/httphelper"
)

type (
	// ArrayPush is what came of pushing one array to the collector.
	ArrayPush struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Keys  int    `json:"keys"`
		Error string `json:"error,omitempty"`
	}

	// oc3ArrayBody is what an array pushes: its name, its type, and the
	// sections its driver reports, by name.
	oc3ArrayBody struct {
		Name string         `json:"name"`
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
)

// pushArrayTimeout bounds the post of one array, whose configuration can
// weigh tens of megabytes.
var pushArrayTimeout = 2 * time.Minute

// PushArrays reports the configuration of the arrays of the cluster to the
// collector.
//
// Each array is posted to the array feed of the oc3 feeder, compressed: the
// sections its driver reports, under the names v2 sent them, which the
// collector parses by type.
//
// One array failing does not stop the others: an array that is down or whose
// credentials have expired must not keep the ones that are reachable from
// being reported. What failed is in the result, and in the error.
func (t Node) PushArrays(ctx context.Context, name string) ([]ArrayPush, error) {
	oc3, err := t.CollectorFeeder()
	if err != nil {
		return nil, err
	}
	// The name is the section, with or without its "array#" prefix, because
	// both spellings reach here: the scheduler names the section it read the
	// schedule from, an operator usually names the array alone. Node.Array
	// accepts both already.
	name = strings.TrimPrefix(name, "array#")

	l := make([]ArrayPush, 0)
	var errs error
	for _, item := range t.ListArrays() {
		if name != "" && item.Name != name {
			continue
		}
		push := ArrayPush{Name: item.Name, Type: item.Type}
		if err := t.pushArray(ctx, oc3, item, &push); err != nil {
			push.Error = err.Error()
			t.Log().Attr("array", item.Name).Warnf("push array %s: %s", item.Name, err)
			errs = errors.Join(errs, fmt.Errorf("array %s: %w", item.Name, err))
		}
		l = append(l, push)
	}
	if name != "" && len(l) == 0 {
		return l, fmt.Errorf("no array found matching %s in the node or cluster config", name)
	}
	return l, errs
}

// pushArray reports one array.
func (t Node) pushArray(ctx context.Context, oc3 *httphelper.T, item ArrayItem, push *ArrayPush) error {
	drv := t.Array(item.Name)
	if drv == nil {
		return fmt.Errorf("no array driver found matching type %s", item.Type)
	}
	return t.pushArrayWithDriver(ctx, oc3, drv, item, push)
}

// pushArrayWithDriver reports one array through a driver already resolved.
func (t Node) pushArrayWithDriver(ctx context.Context, oc3 *httphelper.T, drv array.Driver, item ArrayItem, push *ArrayPush) error {
	reporter, ok := drv.(array.Reporter)
	if !ok {
		return fmt.Errorf("the %s driver does not report its configuration", item.Type)
	}
	data, err := array.Collect(ctx, reporter)
	if err != nil {
		return err
	}
	push.Keys = len(data.Keys)
	body := oc3ArrayBody{Name: item.Name, Type: item.Type, Data: make(map[string]any, len(data.Keys))}
	for i, k := range data.Keys {
		body.Data[k] = data.Values[i]
	}
	return t.postArray(ctx, oc3, body)
}

// postArray posts the configuration of an array to the oc3 array feed,
// compressed.
func (t Node) postArray(ctx context.Context, oc3 *httphelper.T, body oc3ArrayBody) error {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		return fmt.Errorf("encode request body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("compress request body: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, pushArrayTimeout)
	defer cancel()
	method, path := http.MethodPost, oc3path.FeedArray
	req, err := oc3.NewRequestWithContext(ctx, method, path, &buf)
	if err != nil {
		return fmt.Errorf("create collector request %s %s: %w", method, path, err)
	}
	req.Header.Set("Content-Encoding", "gzip")
	resp, err := oc3.Do(req)
	if err != nil {
		return fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return CollectorResponseStatusCheck(resp, method, path, []int{http.StatusAccepted})
}
