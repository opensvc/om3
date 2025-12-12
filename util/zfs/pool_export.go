package zfs

import (
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
)

type (
	poolExportOpts struct {
		Force bool
	}
)

// PoolExportWithForce forcefully unmounts all datasets, using the unmount -f command.
// This option is not supported on Linux.
func PoolExportWithForce() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*poolExportOpts)
		t.Force = true
		return nil
	})
}

func poolExportOptsToArgs(t poolExportOpts) []string {
	l := []string{"export"}
	if t.Force {
		l = append(l, "-f")
	}
	return l
}

func (t *Pool) Export(fopts ...funcopt.O) error {
	opts := &poolExportOpts{}
	funcopt.Apply(opts, fopts...)
	args := append(poolExportOptsToArgs(*opts), t.Name)
	cmd := command.New(
		command.WithName("zpool"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.WarnLevel),
	)
	return cmd.Run()
}
