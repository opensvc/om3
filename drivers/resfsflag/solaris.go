// +build solaris

package resfsflag

import (
	"path/filepath"

	"opensvc.com/opensvc/util/file"
)

func (t T) baseDir() string {
	p := filepath.FromSlash("/system/volatile")
	if file.Exists(p) {
		return filepath.Join(p, "opensvc")
	}
	return filepath.FromSlash("/var/run/opensvc")
}
