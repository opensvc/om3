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
	cmd := exec.Command("zfs", args...)
	cmdStr := cmd.String()
	if opts.Node == "" {
		if t.Log != nil {
			t.Log.Attr("cmd", cmdStr).Debugf("destroy zfs '%s'", t.Name)
		}
		b, err := cmd.CombinedOutput()
		if strings.Contains(string(b), "could not find") {
			return nil
		}
		if strings.Contains(string(b), "does not exist") {
			return nil
		}
		if err != nil {
			t.Log.
				Attr("exitcode", cmd.ProcessState.ExitCode()).
				Attr("cmd", cmdStr).
				Attr("outputs", string(b)).
				Errorf("%s destroy exited with code %d", t.Name, cmd.ProcessState.ExitCode())
		} else {
			if t.Log != nil {
				t.Log.
					Attr("exitcode", cmd.ProcessState.ExitCode()).
					Attr("cmd", cmdStr).
					Infof("%s destroyed", t.Name)
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

	if t.Log != nil {
		t.Log.Debugf("rexec '%s' on node %s", cmdStr, opts.Node)
	}
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
			Attr("exitcode", ec).
			Attr("cmd", cmdStr).
			Attr("peer", opts.Node).
			Attr("outputs", string(b)).
			Errorf("destroy %s on node %s exited with code %d", t.Name, opts.Node, ec)
		return err
	} else {
		if t.Log != nil {
			t.Log.
				Attr("cmd", cmdStr).
				Attr("peer", opts.Node).
				Infof("%s destroyed on node %s", t.Name, opts.Node)
		}
	}
	return nil
}
