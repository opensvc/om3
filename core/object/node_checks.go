package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/exe"
)

// Checks finds and runs the check drivers.
// Results are aggregated and sent to the collector.
func (t Node) Checks(ctx context.Context) (check.ResultSet, error) {
	rootPath := filepath.Join(rawconfig.Paths.Drivers, "check", "chk*")
	customCheckPaths := exe.FindExe(rootPath)
	paths, err := naming.InstalledPaths()
	if err != nil {
		return *check.NewResultSet(), err
	}
	objs, err := NewList(paths.Filter("*/svc/*").Merge(paths.Filter("*/vol/*")), WithVolatile(true))
	if err != nil {
		return *check.NewResultSet(), err
	}
	runner := check.NewRunner(
		check.RunnerWithCustomCheckPaths(customCheckPaths...),
		check.RunnerWithObjects(objs...),
	)
	rs := runner.Do(ctx)
	if err := t.pushChecks(rs); err != nil {
		return *rs, err
	}
	return *rs, nil
}

// oc3ChecksBody is the body of a node checks feed: the results as the check
// drivers report them.
type oc3ChecksBody struct {
	Data []check.Result `json:"data"`

	// Partial says a checker failed during the run, so the results miss
	// some of the checks the node has: the collector updates the checks
	// reported and keeps the others, with their alerts.
	Partial bool `json:"partial,omitempty"`
}

// pushChecks sends the results to the collector, which replaces the checks
// of the node with them: an empty set is sent too, for the checks the node
// no longer reports to go. A partial set, of a run where a checker failed,
// replaces nothing: a transient failure must not delete the last known
// checks of the failed checker, and their alerts.
func (t Node) pushChecks(rs *check.ResultSet) error {
	var (
		method = http.MethodPost
		path   = oc3path.FeedNodeChecks
	)
	oc3, err := t.CollectorFeeder()
	if err != nil {
		return err
	}
	body := oc3ChecksBody{Data: rs.Data, Partial: rs.IsPartial()}
	if body.Data == nil {
		body.Data = []check.Result{}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request body: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultPostCollectorTimeout)
	defer cancel()
	req, err := oc3.NewRequestWithContext(ctx, method, path, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("create collector request %s %s: %w", method, path, err)
	}
	resp, err := oc3.Do(req)
	if err != nil {
		return fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected collector response status code for %s %s: wanted %d got %d: %s",
			method, path, http.StatusAccepted, resp.StatusCode, bytes.TrimSpace(b))
	}
	return nil
}
