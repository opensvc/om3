//go:build darwin

package resfsflag

import "path/filepath"

func (t *T) baseDir() string {
	return filepath.FromSlash("/var/run/opensvc")
}
