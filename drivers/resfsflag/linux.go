// +build linux

package resfsflag

import (
	"path/filepath"
)

func (t T) baseDir() string {
	return filepath.FromSlash("/tmp/opensvc")
}
