package zfs

import (
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/util/command"
)

func (t *Filesystem) Exists() (bool, error) {
	return t.existsWithType("all")
}

func (t *Filesystem) SnapshotExists() (bool, error) {
	return t.existsWithType("snapshot")
}

func (t *Filesystem) existsWithType(s string) (bool, error) {
	cmd := command.New(
		command.WithName("/usr/sbin/zfs"),
		command.WithVarArgs("list", "-t", s, t.Name),
		command.WithLogger(t.Log),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
	)
	err := cmd.Run()
	if err == nil {
		return true, nil
	} else if b := cmd.Stderr(); strings.Contains(string(b), "does not exist") {
		return false, nil
	} else {
		return false, err
	}
}
