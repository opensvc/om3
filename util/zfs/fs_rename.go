package zfs

import (
	"bytes"
	"os/exec"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/util/args"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
)

type (
	fsRenameOpts struct {
		Name    string
		Recurse bool
		Node    string
	}
)

func FilesystemRenameWithNode(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsRenameOpts)
		t.Node = s
		return nil
	})
}

// FilesystemRenameWithRecurse recursively renames all datasets
func FilesystemRenameWithRecurse(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsRenameOpts)
		t.Recurse = v
		return nil
	})
}

func fsRenameOptsToArgs(t fsRenameOpts) []string {
	a := args.New()
	a.Append("rename")
	if t.Recurse {
		a.Append("-r")
	}
	a.Append(t.Name)
	return a.Get()
}

func (t *Filesystem) Rename(dst string, fopts ...funcopt.O) error {
	var b bytes.Buffer
	opts := &fsRenameOpts{Name: t.Name}
	funcopt.Apply(opts, fopts...)
	args := fsRenameOptsToArgs(*opts)
	args = append(args, dst)
	cmd := exec.Command("zfs", args...)
	cmdStr := cmd.String()
	cmd.Stdout = &b
	cmd.Stderr = &b
	if opts.Node == "" || opts.Node == hostname.Hostname() {
		err := cmd.Run()
		if err != nil {
			t.Log.
				Attr("exitcode", cmd.ProcessState.ExitCode()).
				Attr("cmd", cmdStr).
				Attr("outputs", string(b.Bytes())).
				Errorf("%s rename as %s exited with code %d", t.Name, dst, cmd.ProcessState.ExitCode())
		} else {
			t.Log.
				Attr("cmd", cmdStr).
				Infof("%s renamed as %s", t.Name, dst)
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

	session.Stdout = &b
	session.Stderr = &b

	if err := session.Run(cmdStr); err != nil {
		ee := err.(*ssh.ExitError)
		ec := ee.Waitmsg.ExitStatus()
		if ec == 0 {
			return nil
		}
		if strings.Contains(string(b.Bytes()), "does not exist") {
			return nil
		}
		t.Log.
			Attr("exitcode", ec).
			Attr("cmd", cmd).
			Attr("peer", opts.Node).
			Errorf("%s rename as %s on node %s exited with code %d", t.Name, dst, opts.Node, ec)
		return err
	} else {
		if t.Log != nil {
			t.Log.
				Attr("cmd", cmdStr).
				Attr("peer", opts.Node).
				Infof("%s renamed as %s on node %s", t.Name, dst, opts.Node)
		}
	}
	return nil
}
