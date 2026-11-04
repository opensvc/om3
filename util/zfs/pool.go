package zfs

import (
	"context"
	"fmt"
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
	return poolExists(cmd.Run(), cmd.Stderr())
}

// noPoolAnswers are the answers of "zpool list <name>" saying no pool of the
// name exists: none does, or none can, as one of a name zfs reserves, as
// "c0t0d0", or refuses. Any other failure says nothing of the pool, as the
// zfs module not loaded.
var noPoolAnswers = []string{
	"no such pool",
	"name is reserved",
	"invalid character",
	"must begin with a letter",
	"name is too long",
}

// poolExists returns the existence of a pool from the end of "zpool list
// <name>": err its error, stderr what it said.
func poolExists(err error, stderr []byte) (bool, error) {
	if err == nil {
		return true, nil
	}
	for _, s := range noPoolAnswers {
		if strings.Contains(string(stderr), s) {
			return false, nil
		}
	}
	return false, withStderr(err, stderr)
}

// withStderr returns err with what the command said, the reason of its
// failure, which its exit code alone does not tell.
func withStderr(err error, stderr []byte) error {
	if s := strings.TrimSpace(string(stderr)); s != "" {
		return fmt.Errorf("%w: %s", err, s)
	}
	return err
}
