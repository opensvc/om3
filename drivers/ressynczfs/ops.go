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

// run runs the zfs command args on nodename, returning its stdout. A
// dataset that does not exist is reported as notFound.
func (o *execOps) run(nodename string, args ...string) (out []byte, notFound bool, err error) {
	var stdout, stderr bytes.Buffer
	if nodename == "" || nodename == hostname.Hostname() {
		cmd := exec.Command("/usr/sbin/zfs", args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
	} else {
		// err is the one returned: the session run error must not be
		// assigned to an err of this block.
		var client *ssh.Client
		if client, err = o.t.NewSSHClient(nodename); err != nil {
			return nil, false, err
		}
		defer client.Close()
		var session *ssh.Session
		if session, err = client.NewSession(); err != nil {
			return nil, false, err
		}
		defer session.Close()
		session.Stdout = &stdout
		session.Stderr = &stderr
		cmd := exec.Command("/usr/sbin/zfs", args...)
		err = session.Run(cmd.String())
	}
	if err != nil {
		if strings.Contains(stderr.String(), "does not exist") {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("zfs %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), false, nil
}

func (o *execOps) listSnapshots(nodename, dataset string) ([]snapshot, error) {
	b, notFound, err := o.run(nodename, "list", "-H", "-p", "-t", "snapshot", "-d", "1", "-o", "name,guid,createtxg", dataset)
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
	fs := o.t.zfs(name)
	if nodename != "" && nodename != hostname.Hostname() {
		return fs.Destroy(zfs.FilesystemDestroyWithRecurse(o.t.Recursive), zfs.FilesystemDestroyWithNode(nodename))
	}
	return fs.Destroy(zfs.FilesystemDestroyWithRecurse(o.t.Recursive))
}

func (o *execOps) reclaimable(dataset, first, last string) (int64, error) {
	args := []string{"destroy", "-n", "-v", "-p"}
	if o.t.Recursive {
		args = append(args, "-r")
	}
	args = append(args, dataset+"@"+first+"%"+last)
	b, _, err := o.run("", args...)
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
