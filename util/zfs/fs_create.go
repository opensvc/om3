package zfs

import (
	"os/exec"
	"strings"

	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"golang.org/x/crypto/ssh"
)

type (
	fsCreateOpts struct {
		Name           string
		Node           string
		RefQuota       *int64
		Quota          *int64
		RefReservation *int64
		Reservation    *int64
		Args           []string
	}
)

func FilesystemCreateWithNode(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsCreateOpts)
		t.Node = s
		return nil
	})
}

// FilesystemCreateWithArgs defines the shlex split list of arguments to prepend
// to the command.
func FilesystemCreateWithArgs(l []string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsCreateOpts)
		if t.Args == nil {
			t.Args = make([]string, 0)
		}
		t.Args = append(t.Args, l...)
		return nil
	})
}

// FilesystemCreateWithRefQuota Limits the amount of space a dataset can consume.
// This property enforces a hard limit on the amount of space used.
// This hard limit does not include space used by descendents, including file
// systems and snapshots.
func FilesystemCreateWithRefQuota(size *int64) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsCreateOpts)
		t.RefQuota = size
		return nil
	})
}

// FilesystemCreateWithQuota Limits the amount of space a dataset and its
// descendents can consume. This property enforces a hard limit on the amount
// of space used. This includes all space consumed by descendents, including
// file systems and snapshots. Setting a quota on a descendent of a dataset
// that already has a quota does not override the ancestor's quota, but rather
// imposes an additional limit.
func FilesystemCreateWithQuota(size *int64) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsCreateOpts)
		t.Quota = size
		return nil
	})
}

// FilesystemCreateWithRefReservation is the minimum amount of space
// guaranteed to a dataset, not including its descendents. When the amount
// of space used is below this value, the dataset is treated as if it were
// taking up the amount of space specified by refreservation. The
// refreservation reservation is accounted for in the parent datasets' space
// used, and counts against the parent datasets' quotas and reservations.
//
// If refreservation is set, a snapshot is only allowed if there is enough
// free pool space outside of this reservation to accommodate the current
// number of "referenced" bytes in the dataset.
func FilesystemCreateWithRefReservation(size *int64) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsCreateOpts)
		t.RefReservation = size
		return nil
	})
}

// FilesystemCreateWithReservation is the minimum amount of space guaranteed
// to a dataset and its descendents. When the amount of space used is below
// this value, the dataset is treated as if it were taking up the amount of
// space specified by its reservation. Reservations are accounted for in the
// parent datasets' space used, and count against the parent datasets' quotas
// and reservations.
func FilesystemCreateWithReservation(size *int64) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*fsCreateOpts)
		t.Reservation = size
		return nil
	})
}

func fsCreateOptsToArgs(t fsCreateOpts) []string {
	a := args.New()
	a.Append("create", "-p")
	if t.RefQuota != nil {
		a.DropOptionAndMatchingValue("-o", "^refquota=.*")
		a.Append("-o", "refquota="+sizeconv.ExactBSizeCompact(float64(*t.RefQuota)))
	}
	if t.Quota != nil {
		a.DropOptionAndMatchingValue("-o", "^quota=.*")
		a.Append("-o", "quota="+sizeconv.ExactBSizeCompact(float64(*t.Quota)))
	}
	if t.RefReservation != nil {
		a.DropOptionAndMatchingValue("-o", "^refreservation=.*")
		a.Append("-o", "refreservation="+sizeconv.ExactBSizeCompact(float64(*t.RefReservation)))
	}
	if t.Reservation != nil {
		a.DropOptionAndMatchingValue("-o", "^reservation=.*")
		a.Append("-o", "reservation="+sizeconv.ExactBSizeCompact(float64(*t.Reservation)))
	}
	if t.Args != nil {
		a.Append(t.Args...)
	}
	a.Append(t.Name)
	return a.Get()
}

func (t *Filesystem) Create(fopts ...funcopt.O) error {
	opts := &fsCreateOpts{Name: t.Name}
	funcopt.Apply(opts, fopts...)
	args := fsCreateOptsToArgs(*opts)
	cmd := exec.Command("/usr/sbin/zfs", args...)
	if opts.Node == "" {
		if t.Log != nil {
			t.Log.Infof("%s", cmd)
		}
		b, err := cmd.CombinedOutput()
		if strings.Contains(string(b), "dataset already exists") {
			return nil
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
		t.Log.Infof("ssh %s %s", opts.Node, cmd)
	}
	if b, err := session.CombinedOutput(cmd.String()); err != nil {
		if ee, ok := err.(*ssh.ExitError); ok {
			ec := ee.Waitmsg.ExitStatus()
			if ec == 0 {
				return nil
			}
			if strings.Contains(string(b), "dataset already exists") {
				return nil
			}
			if t.Log != nil {
				t.Log.Errorf("ssh %s %s: exited with code %d", opts.Node, cmd, ec)
			}
		}
		return err
	}
	return nil
}
