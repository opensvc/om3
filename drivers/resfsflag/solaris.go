//go:build solaris

package resfsflag

import (
	"path/filepath"

	"github.com/opensvc/om3/v3/util/file"
)

func (t *T) baseDir() string {
	return nodeBaseDir()
}

// nodeBaseDir returns the base directory of the flag files of the node.
func nodeBaseDir() string {
	p := filepath.FromSlash("/system/volatile")
	if file.Exists(p) {
		return filepath.Join(p, "opensvc")
	}
	return filepath.FromSlash("/var/run/opensvc")
}
