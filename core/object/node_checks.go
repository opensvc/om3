package object

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/check"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/exe"
	"opensvc.com/opensvc/util/hostname"

	_ "opensvc.com/opensvc/drivers/chkfsidf"
	_ "opensvc.com/opensvc/drivers/chkfsudf"
)

// OptsNodeChecks is the options of the Checks function.
type OptsNodeChecks struct {
	Global OptsGlobal
}

// Checks finds and runs the check drivers.
// Results are aggregated and sent to the collector.
func (t Node) Checks() (check.ResultSet, error) {
	rootPath := filepath.Join(rawconfig.Paths.Drivers, "check", "chk*")
	customCheckPaths := exe.FindExe(rootPath)
	sel := NewSelection("*/vol/*,*/svc/*", SelectionWithLocal(true))
	objs, err := sel.Objects(WithVolatile(true))
	if err != nil {
		return *check.NewResultSet(), err
	}
	runner := check.NewRunner(
		check.RunnerWithCustomCheckPaths(customCheckPaths...),
		check.RunnerWithObjects(objs...),
	)
	rs := runner.Do()
	if err := t.pushChecks(rs); err != nil {
		return *rs, err
	}
	return *rs, nil
}

func (t Node) pushChecks(rs *check.ResultSet) error {
	client, err := t.collectorFeedClient()
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
		return errors.Errorf("rpc: %s: %s", response.Error.Message, response.Error.Data)
	}
	return nil
}
