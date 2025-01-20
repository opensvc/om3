package zfs

import (
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/rs/zerolog"
)

type (
	poolCreateOpts struct {
		Name  string
		VDevs []string
		Args  []string
	}
)

// PoolCreateWithVDevs defines the list of block devices paths to add to the pool.
func PoolCreateWithVDevs(l []string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*poolCreateOpts)
		if t.VDevs == nil {
			t.VDevs = make([]string, 0)
		}
		t.VDevs = append(t.VDevs, l...)
		return nil
	})
}

// PoolCreateWithArgs defines the shlex split list of arguments to prepend
// to the command.
func PoolCreateWithArgs(l []string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*poolCreateOpts)
		if t.Args == nil {
			t.Args = make([]string, 0)
		}
		t.Args = append(t.Args, l...)
		return nil
	})
}

func poolCreateOptsToArgs(t poolCreateOpts) []string {
	l := []string{"create"}

	// zpool create <options> <name> <vdev>
	//              ^^^^^^^^^
	if t.Args != nil {
		l = append(l, t.Args...)
	}

	// zpool create <options> <name> <vdev>
	//                        ^^^^^^
	l = append(l, t.Name)

	// zpool create <options> <name> <vdev>
	//                               ^^^^^^
	if t.VDevs != nil {
		l = append(l, t.VDevs...)
	}
	return l
}

func (t *Pool) Create(fopts ...funcopt.O) error {
	opts := &poolCreateOpts{Name: t.Name}
	funcopt.Apply(opts, fopts...)
	args := poolCreateOptsToArgs(*opts)
	cmd := command.New(
		command.WithName("zpool"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}
