//go:build solaris

package resfsflag

import (
	"path/filepath"

	"github.com/opensvc/om3/util/file"
)

func (t *T) baseDir() string {
	p := filepath.FromSlash("/system/volatile")
	if file.Exists(p) {
		return filepath.Join(p, "opensvc")
	}
	return filepath.FromSlash("/var/run/opensvc")
}
