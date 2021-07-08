// +build freebsd

package poolshm

import (
	"path/filepath"

	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/df"
	"opensvc.com/opensvc/util/filesystems"
)

func (t T) path() string {
	return filepath.Join(rawconfig.Node.Paths.Var, "pool", "shm")
}

func (t T) Prepare() error {
	fs := filesystems.FromType("tmpfs")
	entries, err := df.TypeMountUsage("tmpfs", t.path())
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fs.Mount("none", t.path(), "")
	}
	return nil
}
