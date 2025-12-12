package zfs

import (
	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/rs/zerolog"
)

type (
	fsUmountOpts struct {
		Name  string
		Force bool
	}
)

// FilesystemUmountWithForce forcefully unmounts the file system,
// even if it is currently in use.  This option is not supported on Linux.
func FilesystemUmountWithForce(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsUmountOpts)
		t.Force = v
		return nil
	})
}

func fsUmountOptsToArgs(t fsUmountOpts) []string {
	a := args.New()
	a.Append("unmount")
	if t.Force {
		a.Append("-f")
	}
	a.Append(t.Name)
	return a.Get()
}

func (t *Filesystem) Umount(fopts ...funcopt.O) error {
	opts := &fsUmountOpts{Name: t.Name}
	funcopt.Apply(opts, fopts...)
	args := fsUmountOptsToArgs(*opts)
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
