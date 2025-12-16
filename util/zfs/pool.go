package zfs

import (
	"context"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	Pool struct {
		Name      string
		Log       *plog.Logger
		LogPrefix string
	}
)

func (t *Pool) Exists(ctx context.Context) (bool, error) {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("zpool"),
		command.WithVarArgs("list", t.Name),
		command.WithLogger(t.Log),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.TraceLevel),
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
