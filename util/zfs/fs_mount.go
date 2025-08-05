package zfs

import (
	"github.com/opensvc/om3/util/args"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/rs/zerolog"
)

type (
	fsMountOpts struct {
		Name    string
		Overlay bool
	}
)

// FilesystemMountWithOverlay performs an overlay mount.
// Allows mounting in non-empty mountpoint.
func FilesystemMountWithOverlay(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsMountOpts)
		t.Overlay = v
		return nil
	})
}

func fsMountOptsToArgs(t fsMountOpts) []string {
	a := args.New()
	a.Append("mount")
	if t.Overlay {
		a.Append("-O")
	}
	a.Append(t.Name)
	return a.Get()
}

func (t *Filesystem) Mount(fopts ...funcopt.O) error {
	opts := &fsMountOpts{Name: t.Name}
	funcopt.Apply(opts, fopts...)
	args := fsMountOptsToArgs(*opts)
	cmd := command.New(
		command.WithName("/usr/sbin/zfs"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}
