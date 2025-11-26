package zfs

import (
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/util/command"
)

func (t *Vol) Exists() (bool, error) {
	cmd := command.New(
		command.WithName("zfs"),
		command.WithVarArgs("list", "-t", "volume", t.Name),
		command.WithLogger(t.Log),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.TraceLevel),
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
