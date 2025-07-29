package zfs

import (
	"os/exec"

	"github.com/opensvc/om3/util/args"
	"github.com/opensvc/om3/util/funcopt"
)

type (
	fsSnapshotOpts struct {
		Name      string
		Recursive bool
		Args      []string
	}
)

// FilesystemSnapshotWithArgs defines the shlex split list of arguments to prepend
// to the command.
func FilesystemSnapshotWithArgs(l []string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsSnapshotOpts)
		if t.Args == nil {
			t.Args = make([]string, 0)
		}
		t.Args = append(t.Args, l...)
		return nil
	})
}

func FilesystemSnapshotWithRecursive(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsSnapshotOpts)
		t.Recursive = v
		return nil
	})
}

func fsSnapshotOptsToArgs(t fsSnapshotOpts) []string {
	a := args.New()
	a.Append("snapshot")
	if t.Recursive {
		a.Append("-r")
	}
	if t.Args != nil {
		a.Append(t.Args...)
	}
	a.Append(t.Name)
	return a.Get()
}

func (t *Filesystem) Snapshot(fopts ...funcopt.O) error {
	opts := &fsSnapshotOpts{Name: t.Name}
	funcopt.Apply(opts, fopts...)
	args := fsSnapshotOptsToArgs(*opts)
	cmd := exec.Command("/usr/sbin/zfs", args...)
	cmdStr := cmd.String()
	if t.Log != nil {
		t.Log.Debugf("exec '%s'", cmdStr)
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Log.
			Attr("outputs", string(b)).
			Errorf("%s: exited with code %d", cmdStr, cmd.ProcessState.ExitCode())
	} else {
		if t.Log != nil {
			t.Log.Infof(cmdStr)
		}
	}
	return err
}
