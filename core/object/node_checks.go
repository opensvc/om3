package object

import (
	"path/filepath"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/check"
	"opensvc.com/opensvc/util/exe"

	_ "opensvc.com/opensvc/drivers/check/fs_i/df"
	_ "opensvc.com/opensvc/drivers/check/fs_u/df"
)

// OptsNodeChecks is the options of the Checks function.
type OptsNodeChecks struct {
	Global OptsGlobal
}

// Checks find and runs the check drivers.
func (t Node) Checks() check.ResultSet {
	rootPath := filepath.Join(config.NodeViper.GetString("paths.drivers"), "check")
	customCheckPaths := exe.FindExe(rootPath)
	rs := check.NewRunner(customCheckPaths).Do()
	return *rs
}
