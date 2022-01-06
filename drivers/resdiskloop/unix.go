//go:build !windows
// +build !windows

package resdiskloop

import (
	"os"
)

func (t T) setFileMode() error {
	t.Log().Info().Msgf("chmod 600 %s", t.File)
	return os.Chmod(t.File, 0600)
}

func (t T) setFileOwner() error {
	t.Log().Info().Msgf("chown 0:0 %s", t.File)
	return os.Chown(t.File, 0, 0)
}
