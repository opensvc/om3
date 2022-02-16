package zfs

import (
	"strings"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/util/command"
)

func (t *Filesystem) Exists() (bool, error) {
	cmd := command.New(
		command.WithName("zfs"),
		command.WithVarArgs("list", "-t", "filesystem", t.Name),
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
