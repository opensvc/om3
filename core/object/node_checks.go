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

	_ "github.com/opensvc/om3/v3/drivers/chkbtrfs"
	_ "github.com/opensvc/om3/v3/drivers/chketh"
	_ "github.com/opensvc/om3/v3/drivers/chkfsidf"
	_ "github.com/opensvc/om3/v3/drivers/chkfsudf"
	_ "github.com/opensvc/om3/v3/drivers/chkfszfs"
	_ "github.com/opensvc/om3/v3/drivers/chkjstat"
	_ "github.com/opensvc/om3/v3/drivers/chklag"
	_ "github.com/opensvc/om3/v3/drivers/chkmcelog"
	_ "github.com/opensvc/om3/v3/drivers/chkmpath"
	_ "github.com/opensvc/om3/v3/drivers/chknuma"
	_ "github.com/opensvc/om3/v3/drivers/chkomreport"
	_ "github.com/opensvc/om3/v3/drivers/chkpowerpath"
	_ "github.com/opensvc/om3/v3/drivers/chkraid"
	_ "github.com/opensvc/om3/v3/drivers/chkvg"
	_ "github.com/opensvc/om3/v3/drivers/chkzpool"
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
}

// pushChecks sends the results to the collector, which replaces the checks
// of the node with them: an empty set is sent too, for the checks the node
// no longer reports to go.
func (t Node) pushChecks(rs *check.ResultSet) error {
	var (
		method = http.MethodPost
		path   = oc3path.FeedNodeChecks
	)
	oc3, err := t.CollectorFeeder()
	if err != nil {
		return err
	}
	body := oc3ChecksBody{Data: rs.Data}
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
