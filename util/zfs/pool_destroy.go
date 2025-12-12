package zfs

import (
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/rs/zerolog"
)

type (
	poolDestroyOpts struct {
		Force bool
	}
)

// PoolDestroyWithForce forces any active datasets contained within the pool to be unmounted.
func PoolDestroyWithForce() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*poolDestroyOpts)
		t.Force = true
		return nil
	})
}

func poolDestroyOptsToArgs(t poolDestroyOpts) []string {
	l := []string{"destroy"}
	if t.Force {
		l = append(l, "-f")
	}
	return l
}

func (t *Pool) Destroy(fopts ...funcopt.O) error {
	opts := &poolDestroyOpts{}
	funcopt.Apply(opts, fopts...)
	args := append(poolDestroyOptsToArgs(*opts), t.Name)
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
