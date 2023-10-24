package zfs

import (
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/util/command"
)

type (
	Pool struct {
		Name      string
		Log       *zerolog.Logger
		LogPrefix string
	}
)

func (t *Pool) Exists() (bool, error) {
	cmd := command.New(
		command.WithName("zpool"),
		command.WithVarArgs("list", t.Name),
		command.WithLogger(t.Log),
		command.WithLogPrefix(t.LogPrefix),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
	)
	err := cmd.Run()
	if err == nil {
		return true, nil
	} else if b := cmd.Stderr(); strings.Contains(string(b), "no such pool") {
		return false, nil
	} else {
		return false, err
	}
}
