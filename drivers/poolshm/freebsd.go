//go:build freebsd || darwin || solaris

package poolshm

import (
	"path/filepath"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

func (t T) path() string {
	return filepath.Join(rawconfig.Paths.Var, "pool", "shm")
}
