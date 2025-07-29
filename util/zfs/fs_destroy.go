package zfs

import (
	"os/exec"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/util/args"
	"github.com/opensvc/om3/util/funcopt"
)

type (
	fsDestroyOpts struct {
		Name            string
		Node            string
		RemoveSnapshots bool
		Recurse         bool
		TryImmediate    bool
	}
)

func FilesystemDestroyWithNode(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsDestroyOpts)
		t.Node = s
		return nil
	})
}

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
	cmd := exec.Command("/usr/sbin/zfs", args...)
	cmdStr := cmd.String()
	if opts.Node == "" {
		b, err := cmd.CombinedOutput()
		if strings.Contains(string(b), "could not find") {
			return nil
		}
		if strings.Contains(string(b), "does not exist") {
			return nil
		}
		if err != nil {
			t.Log.
				Attr("outputs", string(b)).
				Errorf("%s: exited with code %d", cmdStr, cmd.ProcessState.ExitCode())
		} else {
			if t.Log != nil {
				t.Log.
					Attr("exitcode", cmd.ProcessState.ExitCode()).
					Infof(cmdStr)
			}
		}
		return err
	}
	client, err := t.newSSHClient(opts.Node)
	if err != nil {
		return err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	if b, err := session.CombinedOutput(cmdStr); err != nil {
		ee := err.(*ssh.ExitError)
		ec := ee.Waitmsg.ExitStatus()
		if ec == 0 {
			return nil
		}
		if strings.Contains(string(b), "could not find") {
			return nil
		}
		if strings.Contains(string(b), "does not exist") {
			return nil
		}
		t.Log.
			Attr("outputs", string(b)).
			Errorf("ssh %s %s: exited with code %d", opts.Node, cmdStr, ec)
		return err
	} else {
		if t.Log != nil {
			t.Log.Infof("ssh %s %s", opts.Node, cmdStr)
		}
	}
	return nil
}
