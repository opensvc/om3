package object

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/exe"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/chkfsidf"
	_ "github.com/opensvc/om3/v3/drivers/chkfsudf"
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

func (t Node) pushChecks(rs *check.ResultSet) error {
	client, err := t.CollectorFeedClient()
	if err != nil {
		return err
	}
	vars := []string{
		"chk_nodename",
		"chk_svcname",
		"chk_type",
		"chk_instance",
		"chk_value",
		"chk_updated",
	}
	vals := make([][]string, rs.Len())
	hn := hostname.Hostname()
	now := time.Now().Format("2006-01-02 15:04:05")
	for i, e := range rs.Data {
		vals[i] = []string{
			hn,
			e.Path,
			e.DriverGroup,
			e.Instance,
			fmt.Sprint(e.Value),
			now,
		}
	}
	if response, err := client.Call("push_checks", vars, vals); err != nil {
		return err
	} else if response.Error != nil {
		return fmt.Errorf("rpc: %s: %s", response.Error.Message, response.Error.Data)
	}
	return nil
}
