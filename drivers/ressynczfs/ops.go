package ressynczfs

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/zfs"
)

// execOps runs the zfs commands of a sync, on the local node or over ssh.
type execOps struct {
	t *T
}

// zfsRun runs the zfs command args on nodename, over the connection of the
// run to it, returning its stdout. A dataset that does not exist is reported as
// notFound.
func (t *T) zfsRun(nodename string, args ...string) (out []byte, notFound bool, err error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("/usr/sbin/zfs", args...)
	if nodename == "" || nodename == hostname.Hostname() {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
	} else {
		var session *ssh.Session
		if session, err = t.newSession(nodename); err != nil {
			return nil, false, err
		}
		defer session.Close()
		session.Stdout = &stdout
		session.Stderr = &stderr
		err = session.Run(cmd.String())
	}
	if err != nil {
		if s := stderr.String(); strings.Contains(s, "does not exist") || strings.Contains(s, "could not find") {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("zfs %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), false, nil
}

// destroySnapshot destroys the snapshot name on nodename, a snapshot that
// does not exist being destroyed already.
func (t *T) destroySnapshot(nodename, name string) error {
	if nodename == "" || nodename == hostname.Hostname() {
		return t.zfs(name).Destroy(zfs.FilesystemDestroyWithRecurse(t.Recursive))
	}
	args := []string{"destroy"}
	if t.Recursive {
		args = append(args, "-R")
	}
	args = append(args, name)
	if _, _, err := t.zfsRun(nodename, args...); err != nil {
		return err
	}
	t.Log().Infof("ssh %s /usr/sbin/zfs %s", nodename, strings.Join(args, " "))
	return nil
}

func (o *execOps) listSnapshots(nodename, dataset string) ([]snapshot, error) {
	b, notFound, err := o.t.zfsRun(nodename, "list", "-H", "-p", "-t", "snapshot", "-d", "1", "-o", "name,guid,createtxg", dataset)
	if err != nil || notFound {
		return nil, err
	}
	return parseSnapshots(b)
}

func parseSnapshots(b []byte) ([]snapshot, error) {
	var l []snapshot
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected zfs list line: %q", line)
		}
		txg, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unexpected createtxg in zfs list line: %q", line)
		}
		l = append(l, snapshot{Name: fields[0], GUID: fields[1], CreateTxg: txg})
	}
	return l, nil
}

func (o *execOps) takeSnapshot(name string) error {
	return o.t.zfs(name).Snapshot(zfs.FilesystemSnapshotWithRecursive(o.t.Recursive))
}

func (o *execOps) destroySnapshot(nodename, name string) error {
	return o.t.destroySnapshot(nodename, name)
}

func (o *execOps) reclaimable(dataset, first, last string) (int64, error) {
	args := []string{"destroy", "-n", "-v", "-p"}
	if o.t.Recursive {
		args = append(args, "-r")
	}
	args = append(args, dataset+"@"+first+"%"+last)
	b, _, err := o.t.zfsRun("", args...)
	if err != nil {
		return 0, err
	}
	return parseReclaim(b)
}

// parseReclaim reads the space a dry run of zfs destroy -vp says it would
// free.
func parseReclaim(b []byte) (int64, error) {
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "reclaim\t"); ok {
			return strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		}
	}
	return 0, fmt.Errorf("no reclaim line in the zfs destroy dry run output")
}

func (o *execOps) available(pool string) (int64, error) {
	s, err := o.t.zfs(pool).GetProperty("available")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(s, 10, 64)
}

func (o *execOps) sendFull(ctx context.Context, nodename, snap string) error {
	return o.t.sendInitial(ctx, nodename, snap)
}

func (o *execOps) sendIncremental(ctx context.Context, nodename, base, snap string) error {
	return o.t.sendIncremental(ctx, nodename, base, snap)
}
