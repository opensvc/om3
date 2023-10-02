package zfs

import (
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/util/args"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/funcopt"
)

type (
	fsDestroyOpts struct {
		Name            string
		RemoveSnapshots bool
		Recurse         bool
		TryImmediate    bool
	}
)

// FilesystemDestroyWithRemoveSnapshots forces an unmount of any file systems using the
// unmount -f command.  This option has no effect on non-file systems or
// unmounted file systems.
// TODO: fix above doc ?
func FilesystemDestroyWithRemoveSnapshots(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsDestroyOpts)
		t.RemoveSnapshots = true
		return nil
	})
}

// FilesystemDestroyWithRecurse recursively destroys all clones of these snapshots,
// including the clones, snapshots, and children.  If this flag is specified,
// the FilesystemDestroyWithTryImmediate flag will have no effect.
func FilesystemDestroyWithRecurse(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsDestroyOpts)
		t.Recurse = v
		return nil
	})
}

// FilesystemDestroyWithTryImmediate destroys immediately.
// If a snapshot cannot be destroyed now, mark it for deferred destruction.
func FilesystemDestroyWithTryImmediate(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsDestroyOpts)
		t.TryImmediate = v
		return nil
	})
}

func fsDestroyOptsToArgs(t fsDestroyOpts) []string {
	a := args.New()
	a.Append("destroy")
	if t.RemoveSnapshots {
		a.Append("-r")
	}
	if t.Recurse {
		a.Append("-R")
	}
	if t.TryImmediate {
		a.Append("-d")
	}
	a.Append(t.Name)
	return a.Get()
}

func (t *Filesystem) Destroy(fopts ...funcopt.O) error {
	opts := &fsDestroyOpts{Name: t.Name}
	funcopt.Apply(opts, fopts...)
	args := fsDestroyOptsToArgs(*opts)
	cmd := command.New(
		command.WithName("zfs"),
		command.WithArgs(args),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}
