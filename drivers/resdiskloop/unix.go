//go:build !windows

package resdiskloop

import (
	"os"
)

func (t *T) setFileMode() error {
	t.Log().Infof("chmod 600 %s", t.File)
	return os.Chmod(t.File, 0600)
}

func (t *T) setFileOwner() error {
	t.Log().Infof("chown 0:0 %s", t.File)
	return os.Chown(t.File, 0, 0)
}
