package object

import (
	"path/filepath"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/check"
	"opensvc.com/opensvc/util/exe"

	_ "opensvc.com/opensvc/drivers/chkfsidf"
	_ "opensvc.com/opensvc/drivers/chkfsudf"
)

// OptsNodeChecks is the options of the Checks function.
type OptsNodeChecks struct {
	Global OptsGlobal
}

// Checks find and runs the check drivers.
func (t Node) Checks() check.ResultSet {
	rootPath := filepath.Join(config.NodeViper.GetString("paths.drivers"), "check", "chk*")
	customCheckPaths := exe.FindExe(rootPath)
	rs := check.NewRunner(customCheckPaths).Do()
	return *rs
}
