// +build freebsd darwin solaris

package poolshm

import (
	"path/filepath"

	"opensvc.com/opensvc/core/rawconfig"
)

func (t T) path() string {
	return filepath.Join(rawconfig.Paths.Var, "pool", "shm")
}
