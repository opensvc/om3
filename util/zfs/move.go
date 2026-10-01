package zfs

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/util/plog"
)

// MoveSnapshotPrefix begins the name of the snapshots a move sends. The
// snapshots of other names, those of a sync.zfs among them, are serving an
// increment as a base, and are never destroyed by a move.
const MoveSnapshotPrefix = "osvc_move_"

type (
	// Mover copies a dataset, a filesystem or a volume, to the node a
	// container is moving to, so the files and devices the container
	// opens exist there when its disks are mirrored to them.
	//
	// The copy is a snapshot sent incrementally from the newest snapshot
	// both nodes hold, the one a previous move or a sync.zfs left, and in
	// full when there is none. The destination receives it unmounted.
	Mover struct {
		// Name is the dataset to copy, the same on both nodes.
		Name string

		// Node is the node to copy it to, which Client is connected to.
		Node   string
		Client *ssh.Client

		Log *plog.Logger

		run moveRunner
	}

	// moveRunner runs the zfs commands of a move, here and on the
	// destination.
	moveRunner interface {
		local(ctx context.Context, args ...string) ([]byte, error)
		remote(ctx context.Context, cmdline string) ([]byte, error)
		pipe(ctx context.Context, send []string, receive string) error
	}

	sshMoveRunner struct {
		client *ssh.Client
	}

	moveSnapshot struct {
		Name      string
		GUID      string
		CreateTxg uint64
	}
)

func (t *Mover) runner() moveRunner {
	if t.run == nil {
		t.run = &sshMoveRunner{client: t.Client}
	}
	return t.run
}

func (t *Mover) infof(format string, args ...any) {
	if t.Log != nil {
		t.Log.Infof(format, args...)
	}
}

// Send snapshots the dataset and sends the snapshot to the destination,
// and returns the snapshot name.
//
// A destination that holds the dataset with no snapshot in common is
// refused: receiving a full copy over it would destroy what it holds, which
// may be the only copy of something.
func (t *Mover) Send(ctx context.Context) (string, error) {
	r := t.runner()
	remoteSnaps, remoteHas, err := t.remoteSnapshots(ctx)
	if err != nil {
		return "", err
	}
	localSnaps, err := t.localSnapshots(ctx)
	if err != nil {
		return "", err
	}
	base := commonBase(localSnaps, remoteSnaps)
	if base == "" && remoteHas {
		return "", fmt.Errorf("%s holds %s with no snapshot in common with this node: destroy it there, or replicate it, for a move to send it", t.Node, t.Name)
	}

	snap := t.Name + "@" + MoveSnapshotPrefix + time.Now().UTC().Format("20060102T150405.000000000Z")
	if _, err := r.local(ctx, "snapshot", snap); err != nil {
		return "", err
	}
	t.infof("zfs snapshot %s", snap)

	var send []string
	receive := []string{"/usr/sbin/zfs", "receive", "-u"}
	if base == "" {
		send = []string{"/usr/sbin/zfs", "send", "-p", snap}
		if parent := path.Dir(t.Name); parent != "." && strings.Contains(parent, "/") {
			// A receive creates the dataset, not its parents.
			if _, err := r.remote(ctx, "/usr/sbin/zfs create -p "+parent); err != nil {
				return "", err
			}
		}
	} else {
		send = []string{"/usr/sbin/zfs", "send", "-p", "-i", base, snap}
		// The destination is rolled back to the base, which a mount or an
		// access time update there has moved it away from.
		receive = append(receive, "-F")
	}
	receive = append(receive, t.Name)
	receiveCmdline := strings.Join(receive, " ")
	t.infof("%s | ssh %s %s", strings.Join(send, " "), t.Node, receiveCmdline)
	if err := r.pipe(ctx, send, receiveCmdline); err != nil {
		return "", fmt.Errorf("send %s to %s: %w", snap, t.Node, err)
	}
	return snap, nil
}

// Prune destroys the move snapshots of the dataset older than keep, here and
// on the destination. The one kept is the base of the next move.
func (t *Mover) Prune(ctx context.Context, keep string) error {
	r := t.runner()
	localSnaps, err := t.localSnapshots(ctx)
	if err != nil {
		return err
	}
	remoteSnaps, _, err := t.remoteSnapshots(ctx)
	if err != nil {
		return err
	}
	var errs []string
	for _, s := range staleMoveSnapshots(localSnaps, keep) {
		if _, err := r.local(ctx, "destroy", s); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		t.infof("zfs destroy %s", s)
	}
	for _, s := range staleMoveSnapshots(remoteSnaps, keep) {
		if _, err := r.remote(ctx, "/usr/sbin/zfs destroy "+s); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		t.infof("ssh %s zfs destroy %s", t.Node, s)
	}
	if len(errs) > 0 {
		return fmt.Errorf("prune the move snapshots of %s: %s", t.Name, strings.Join(errs, "; "))
	}
	return nil
}

// Remote runs a command on the destination.
func (t *Mover) Remote(ctx context.Context, cmdline string) error {
	if _, err := t.runner().remote(ctx, cmdline); err != nil {
		return err
	}
	t.infof("ssh %s %s", t.Node, cmdline)
	return nil
}

func (t *Mover) localSnapshots(ctx context.Context) ([]moveSnapshot, error) {
	b, err := t.runner().local(ctx, snapshotListArgs(t.Name)...)
	if err != nil {
		return nil, err
	}
	return parseMoveSnapshots(b)
}

// remoteSnapshots returns the snapshots of the dataset on the destination,
// and whether it holds the dataset at all.
func (t *Mover) remoteSnapshots(ctx context.Context) ([]moveSnapshot, bool, error) {
	r := t.runner()
	b, err := r.remote(ctx, "/usr/sbin/zfs list -H -o name "+t.Name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, false, nil
	}
	b, err = r.remote(ctx, "/usr/sbin/zfs "+strings.Join(snapshotListArgs(t.Name), " "))
	if err != nil {
		return nil, true, err
	}
	l, err := parseMoveSnapshots(b)
	return l, true, err
}

func snapshotListArgs(name string) []string {
	return []string{"list", "-H", "-p", "-t", "snapshot", "-d", "1", "-o", "name,guid,createtxg", name}
}

func parseMoveSnapshots(b []byte) ([]moveSnapshot, error) {
	var l []moveSnapshot
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
		l = append(l, moveSnapshot{Name: fields[0], GUID: fields[1], CreateTxg: txg})
	}
	return l, nil
}

// commonBase returns the newest local snapshot the destination holds too,
// told by its guid, which a send and a receive keep. Empty when there is
// none.
func commonBase(local, remote []moveSnapshot) string {
	guids := make(map[string]bool, len(remote))
	for _, s := range remote {
		guids[s.GUID] = true
	}
	var base *moveSnapshot
	for i, s := range local {
		if !guids[s.GUID] {
			continue
		}
		if base == nil || s.CreateTxg > base.CreateTxg {
			base = &local[i]
		}
	}
	if base == nil {
		return ""
	}
	return base.Name
}

// staleMoveSnapshots returns the move snapshots of l but keep.
func staleMoveSnapshots(l []moveSnapshot, keep string) []string {
	_, keepName, _ := strings.Cut(keep, "@")
	var stale []string
	for _, s := range l {
		_, name, _ := strings.Cut(s.Name, "@")
		if strings.HasPrefix(name, MoveSnapshotPrefix) && name != keepName {
			stale = append(stale, s.Name)
		}
	}
	slices.Sort(stale)
	return stale
}

func (t *sshMoveRunner) local(ctx context.Context, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "/usr/sbin/zfs", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", cmd, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func (t *sshMoveRunner) remote(ctx context.Context, cmdline string) ([]byte, error) {
	session, err := t.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	stop := context.AfterFunc(ctx, func() { _ = session.Close() })
	defer stop()
	if err := session.Run(cmdline); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", cmdline, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func (t *sshMoveRunner) pipe(ctx context.Context, send []string, receive string) error {
	session, err := t.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	var sendErr, receiveErr bytes.Buffer
	cmd := exec.CommandContext(ctx, send[0], send[1:]...)
	cmd.Stderr = &sendErr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	session.Stdin = stdout
	session.Stderr = &receiveErr
	if err := cmd.Start(); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = session.Close() })
	defer stop()
	receiveRunErr := session.Run(receive)
	sendRunErr := cmd.Wait()
	switch {
	case receiveRunErr != nil:
		return fmt.Errorf("%s: %w: %s", receive, receiveRunErr, strings.TrimSpace(receiveErr.String()))
	case sendRunErr != nil:
		return fmt.Errorf("%s: %w: %s", cmd, sendRunErr, strings.TrimSpace(sendErr.String()))
	}
	return nil
}
