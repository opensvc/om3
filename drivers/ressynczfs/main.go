package ressynczfs

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/nodesinfo"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/drivers/ressync"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/proc"
	"github.com/opensvc/om3/v3/util/zfs"
)

// T is the driver structure.
type (
	T struct {
		ressync.T
		resource.SSH
		Src          string
		Dst          string
		Target       []string
		Intermediary bool
		Recursive    bool
		Nodes        []string
		DRPNodes     []string
		ObjectID     uuid.UUID
		Timeout      *time.Duration
		Topology     topology.T
		User         string
		MaxLagAge    *time.Duration
		MaxLagSize   string

		ops   zfsOps
		conns sshClients
	}

	modeT uint
)

const (
	modeFull modeT = iota
	modeIncr

	lockName = "sync"
)

func New() resource.Driver {
	return &T{}
}

func (t *T) Full(ctx context.Context) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	target := actioncontext.Target(ctx)
	cancel, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer cancel()
	return t.lockedSync(ctx, modeFull, target)
}

func (t *T) Update(ctx context.Context) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	target := actioncontext.Target(ctx)
	cancel, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer cancel()
	return t.lockedSync(ctx, modeIncr, target)
}

func (t *T) lockedSync(ctx context.Context, mode modeT, target []string) (err error) {
	isCron := actioncontext.IsCron(ctx)

	if t.isFlexAndNotPrimary() {
		return fmt.Errorf("this flex instance is not primary. only %s can sync", t.Nodes[0])
	}

	if v, reason := t.IsInstanceSufficientlyStarted(ctx); !v {
		return fmt.Errorf("the instance is not sufficiently started (%s). refuse to sync to protect the data of the started remote instance", reason)
	}

	nodenames, err := t.SelectPeernames(target, t.Target, t.Nodes, t.DRPNodes)
	if err != nil {
		return err
	}
	if len(nodenames) == 0 {
		t.Log().Infof("no peer to sync")
		return nil
	}
	done, err := t.StartRun()
	if err != nil {
		return err
	}
	defer done()

	state, err := t.loadSyncState()
	if err != nil {
		return err
	}
	now := time.Now()
	snapName := t.newSnapName(now)
	if err := t.ops.takeSnapshot(snapName); err != nil {
		return err
	}
	l, err := t.ops.listSnapshots(hostname.Hostname(), t.Src)
	if err != nil {
		return err
	}
	local := t.ownSnapshots(l)
	if len(local) == 0 || local[len(local)-1].Name != snapName {
		return fmt.Errorf("%s is not the newest snapshot of %s", snapName, t.Src)
	}
	snap := local[len(local)-1]
	var previous *snapshot
	if len(local) > 1 {
		previous = &local[len(local)-2]
	}
	states := state.validPeers(previous)
	if len(state.Peers) > 0 && len(states) == 0 {
		t.Log().Infof("another node was the source since the last run of this one: forget what it knew of the peers")
	}

	// The peers of the configuration, not only the ones of this run: the
	// base of a peer not synced this run is kept too.
	peers := t.GetTargetPeernames(t.Target, t.Nodes, t.DRPNodes)
	t.seedLegacyStates(local, peers, states)

	// The peers are synced at once, each on its own: one failing, or slow,
	// does not hold the others back, and all the failures are reported.
	defer t.closeSSHClients()
	var (
		wg       sync.WaitGroup
		peerErrs = make([]error, len(nodenames))
		results  = make([]*peerState, len(nodenames))
	)
	for i, nodename := range nodenames {
		if err := t.isSendAllowedToPeerEnv(nodename); err != nil {
			if isCron {
				t.Log().Tracef("%s", err)
			} else {
				t.Log().Infof("%s", err)
			}
			continue
		}
		st := states[nodename]
		results[i] = &st
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := t.syncPeer(ctx, mode, nodename, local, snap, &st, now); err != nil {
				peerErrs[i] = err
				return
			}
			if err := t.WritePeerLastSync(ctx, nodename); err != nil {
				peerErrs[i] = fmt.Errorf("%s: write last sync: %w", nodename, err)
			}
		}()
	}
	wg.Wait()
	for i, nodename := range nodenames {
		if results[i] != nil {
			states[nodename] = *results[i]
		}
	}
	errs := errors.Join(peerErrs...)
	if err := t.saveSyncState(syncState{LastSnapGUID: snap.GUID, Peers: states}); err != nil {
		return errors.Join(errs, err)
	}
	if err := t.pruneLocal(local, peers, states); err != nil {
		errs = errors.Join(errs, fmt.Errorf("destroy snapshots no peer needs: %w", err))
	}
	return errs
}

func (t *T) sendIncrementalLocal(ctx context.Context, nodename, base, snap string) error {
	nfoWriter := t.Log().Writer(zerolog.InfoLevel)
	errWriter := t.Log().Writer(zerolog.ErrorLevel)

	args := t.sendIncrementalCmd(base, snap)
	cmd := exec.Command(args[0], args[1:]...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe for zfs send: %w", err)
	}
	defer stdoutPipe.Close()

	discardFirst, dst := t.receiveDst()
	rargs := t.receiveCmd([]string{"mountpoint", "canmount"}, discardFirst, dst)
	rcmd := exec.CommandContext(ctx, rargs[0], rargs[1:]...)
	rstdinPipe, err := rcmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("error creating stdin pipe for zfs recv: %w", err)
	}

	cmd.Stderr = errWriter
	rcmd.Stdout = nfoWriter
	rcmd.Stderr = errWriter

	rcmdStr := rcmd.String()
	cmdStr := cmd.String()
	t.Log().Infof("%s | %s", cmdStr, rcmdStr)

	if err := rcmd.Start(); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	stats := ressync.NewStats(nodename)

	if _, err := t.CopyWithStats(ctx, rstdinPipe, stdoutPipe, stats); err != nil {
		return err
	}

	if err := rcmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			ec := ee.ExitCode()
			t.Log().Errorf("exec '%s' on localhost exited with code %d", cmdStr, ec)
		}
		return err
	}

	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			ec := ee.ExitCode()
			t.Log().Errorf("exec '%s' on localhost exited with code %d", cmdStr, ec)
		}
		return err
	}
	return nil
}

func (t *T) sendIncremental(ctx context.Context, nodename, base, snap string) error {
	if hostname.Hostname() == nodename {
		return t.sendIncrementalLocal(ctx, nodename, base, snap)
	} else {
		return t.sendIncrementalTo(ctx, nodename, base, snap)
	}
}

func (t *T) sendIncrementalTo(ctx context.Context, nodename, base, snap string) error {
	nfoWriter := t.Log().Writer(zerolog.InfoLevel)
	errWriter := t.Log().Writer(zerolog.ErrorLevel)

	args := t.sendIncrementalCmd(base, snap)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	session, err := t.newSession(nodename)
	if err != nil {
		return err
	}
	defer session.Close()

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return err
	}
	defer stdinPipe.Close()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdoutPipe.Close()

	cmd.Stderr = errWriter
	session.Stderr = errWriter
	session.Stdout = nfoWriter

	discardFirst, dst := t.receiveDst()
	rargs := t.receiveCmd(nil, discardFirst, dst)
	rcmdStr := exec.Command(rargs[0], rargs[1:]...).String()
	cmdStr := cmd.String()
	t.Log().Infof("%s | ssh %s '%s'", cmdStr, nodename, rcmdStr)
	if err := session.Start(rcmdStr); err != nil {
		ee := err.(*ssh.ExitError)
		ec := ee.Waitmsg.ExitStatus()
		t.Log().Errorf("rexec '%s' on host %s exited with code %d", rcmdStr, nodename, ec)
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	stats := ressync.NewStats(nodename)
	if _, err := t.CopyWithStats(ctx, stdinPipe, stdoutPipe, stats); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ec := ee.ExitCode()
			t.Log().Errorf("exec '%s' on host %s exited with code %d: %s", cmdStr, nodename, ec)
		}
		return err
	}

	if err := session.Wait(); err != nil {
		return err
	}

	return nil
}

func (t *T) sendInitial(ctx context.Context, nodename, snap string) error {
	if err := t.destroySnapshot(nodename, t.Dst+"@%"); err != nil {
		return err
	}
	if hostname.Hostname() == nodename {
		return t.sendInitialLocal(ctx, nodename, snap)
	} else {
		return t.sendInitialTo(ctx, nodename, snap)
	}
}

func (t *T) sendInitialLocal(ctx context.Context, nodename, snap string) error {
	nfoWriter := t.Log().Writer(zerolog.InfoLevel)
	errWriter := t.Log().Writer(zerolog.ErrorLevel)

	args := t.sendInitialCmd(snap)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe for zfs send: %w", err)
	}
	defer stdoutPipe.Close()

	discardFirst, dst := t.receiveDst()
	if !discardFirst {
		if err := t.zfs(dst).Create(); err != nil {
			return err
		}
	}
	rargs := t.receiveCmd([]string{"mountpoint", "canmount"}, discardFirst, dst)
	rcmd := exec.CommandContext(ctx, rargs[0], rargs[1:]...)
	rstdinPipe, err := rcmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("error creating stdin pipe for zfs recv: %w", err)
	}

	cmd.Stderr = errWriter
	rcmd.Stdout = nfoWriter
	rcmd.Stderr = errWriter

	rcmdStr := rcmd.String()
	cmdStr := cmd.String()
	t.Log().Infof("%s | %s", cmdStr, rcmdStr)

	if err := rcmd.Start(); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	stats := ressync.NewStats(nodename)

	if _, err := t.CopyWithStats(ctx, rstdinPipe, stdoutPipe, stats); err != nil {
		return err
	}

	if err := rcmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			ec := ee.ExitCode()
			t.Log().
				Attr("exitcode", ec).
				Errorf("exec '%s' on %s exited with code %d", rcmdStr, nodename, ec)
		}
		return err
	}

	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			ec := ee.ExitCode()
			t.Log().
				Attr("exitcode", ec).
				Errorf("exec '%s' on localhost exited with code %d", cmdStr, ec)
		}
		return err
	}
	return nil
}

func (t *T) sendInitialTo(ctx context.Context, nodename, snap string) error {
	nfoWriter := t.Log().Writer(zerolog.InfoLevel)
	errWriter := t.Log().Writer(zerolog.ErrorLevel)

	args := t.sendInitialCmd(snap)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	session, err := t.newSession(nodename)
	if err != nil {
		return err
	}
	defer session.Close()

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return err
	}
	defer stdinPipe.Close()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdoutPipe.Close()

	cmd.Stderr = errWriter
	session.Stdout = nfoWriter
	session.Stderr = errWriter

	discardFirst, dst := t.receiveDst()
	if !discardFirst {
		if err := t.zfs(dst).Create(zfs.FilesystemCreateWithNode(nodename)); err != nil {
			return err
		}
	}
	rargs := t.receiveCmd(nil, discardFirst, dst)
	rcmdStr := exec.Command(rargs[0], rargs[1:]...).String()
	cmdStr := cmd.String()
	t.Log().Infof("%s | ssh %s '%s'", cmdStr, nodename, rcmdStr)
	if err := session.Start(rcmdStr); err != nil {
		ee := err.(*ssh.ExitError)
		ec := ee.Waitmsg.ExitStatus()
		t.Log().
			Attr("host", nodename).
			Errorf("rexec '%s' on host %s exited with code %d", rcmdStr, nodename, ec)
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	stats := ressync.NewStats(nodename)
	if _, err := t.CopyWithStats(ctx, stdinPipe, stdoutPipe, stats); err != nil {
		return err
	}

	if err := session.Wait(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ec := ee.ExitCode()
			t.Log().Errorf("exec '%s' on host %s exited with code %d", cmdStr, nodename, ec)
		}
		return err
	}

	return nil
}

func (t *T) sendInitialCmd(snap string) []string {
	cmd := []string{"/usr/sbin/zfs", "send"}
	if t.Recursive {
		cmd = append(cmd, "-R")
	} else {
		cmd = append(cmd, "-p")
	}
	cmd = append(cmd, snap)
	return cmd
}

func (t *T) sendIncrementalCmd(base, snap string) []string {
	cmd := []string{"/usr/sbin/zfs", "send"}
	if t.Recursive {
		cmd = append(cmd, "-R")
	}
	if t.Intermediary {
		cmd = append(cmd, "-I")
	} else {
		cmd = append(cmd, "-i")
	}
	cmd = append(cmd, base, snap)
	return cmd
}

func getUpperFs(s string) string {
	return filepath.Dir(s)
}

func (t *T) receiveDst() (discardFirst bool, dst string) {
	srcPool := t.zfs(t.Src).PoolName()
	dstPool := t.zfs(t.Dst).PoolName()
	if t.Src == t.Dst || (t.Src == srcPool && t.Dst == dstPool) {
		return true, dstPool
	} else {
		upperFs := getUpperFs(t.Dst)
		return false, upperFs
	}
}

// receiveCmd is the zfs receive command of a peer, or of the local node when
// inherit is set.
//
// A peer receives with -u: its copy is left unmounted, as the fs resource of
// the object mounts it on the node the object starts on. Mounted by the
// receive, the copy on a passive node would have that fs resource read up
// there, and the node taken for the one the data is replicated from.
func (t *T) receiveCmd(inherit []string, discardFirst bool, dst string) []string {
	cmd := []string{"/usr/sbin/zfs", "receive"}
	if inherit == nil {
		cmd = append(cmd, "-u")
	}
	for _, prop := range inherit {
		cmd = append(cmd, "-x", prop)
	}
	if discardFirst {
		cmd = append(cmd, "-dF", dst)
	} else {
		cmd = append(cmd, "-eF", dst)
	}
	return cmd
}

func (t *T) Kill(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	var isSourceNode bool
	if v, _ := t.IsInstanceSufficientlyStarted(ctx); !v {
		isSourceNode = false
	} else if t.isFlexAndNotPrimary() {
		isSourceNode = false
	} else {
		isSourceNode = true
	}
	nodenames := t.getTargetNodenames(isSourceNode)
	state := t.StatusLastSync(nodenames)
	if isSourceNode {
		state.Add(t.statusStranded(nodenames))
	}
	return state
}

// statusStranded warns about the peers the source can send no increment to
// until an administrator asks a full copy for them.
func (t *T) statusStranded(nodenames []string) status.T {
	state, err := t.loadSyncState()
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	if len(state.Peers) == 0 {
		return status.Undef
	}
	l, err := t.ops.listSnapshots(hostname.Hostname(), t.Src)
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	var newest *snapshot
	if local := t.ownSnapshots(l); len(local) > 0 {
		newest = &local[len(local)-1]
	}
	states := state.validPeers(newest)
	result := status.Undef
	for _, nodename := range nodenames {
		st, ok := states[nodename]
		if !ok || st.StrandedAt.IsZero() {
			continue
		}
		t.StatusLog().Warn("%s: not synced since %s: %s: run '%s'", nodename, st.StrandedAt.Format(time.RFC3339), st.StrandedReason, t.fullCommand(nodename))
		result.Add(status.Warn)
	}
	return result
}

func (t *T) running(ctx context.Context) bool {
	return false
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	switch {
	case t.Src != "" && len(t.Target) > 0:
		return t.Src + " to " + strings.Join(t.Target, " ")
	case t.Src != "":
		return t.Src + " to void"
	case len(t.Target) > 0:
		return "nothing to " + strings.Join(t.Target, " ")
	default:
		return ""
	}
}

func (t *T) getRunning(cmdArgs []string) (proc.L, error) {
	procs, err := proc.All()
	if err != nil {
		return procs, err
	}
	procs = procs.FilterByEnv("OPENSVC_ID", t.ObjectID.String())
	procs = procs.FilterByEnv("OPENSVC_RID", t.RID())
	return procs, nil
}

func (t *T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action:                   "update",
		Option:                   "schedule",
		Base:                     "",
		RequireReplicationSource: true,
		Require:                  t.UpdateRequires,
	}
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) Configure() error {
	if t.ops == nil {
		t.ops = &execOps{t: t}
	}
	return nil
}

func (t *T) zfs(name string) *zfs.Filesystem {
	return &zfs.Filesystem{Name: name, Log: t.Log(), SSHKeyFile: t.GetSSHKeyFile()}
}

func (t *T) user() string {
	if t.User != "" {
		return t.User
	} else {
		return "root"
	}
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	target := sort.StringSlice(t.Target)
	sort.Sort(target)
	m := resource.InfoKeys{
		{Key: "src", Value: t.Src},
		{Key: "dst", Value: t.Dst},
		{Key: "recursive", Value: fmt.Sprintf("%v", t.Recursive)},
		{Key: "target", Value: strings.Join(target, " ")},
	}
	if t.Timeout != nil {
		m = append(m, resource.InfoKey{Key: "timeout", Value: fmt.Sprintf("%s", t.Timeout)})
	}
	return m, nil
}

func (t *T) isFlexAndNotPrimary() bool {
	if t.Topology != topology.Flex {
		return false
	}
	if hostname.Hostname() == t.Nodes[0] {
		return false
	}
	return true
}

func (t *T) isSendAllowedToPeerEnv(nodename string) error {
	var localEnv, peerEnv string
	nodesInfo, err := nodesinfo.Load()
	if err != nil {
		return fmt.Errorf("get nodes info: %w", err)
	}
	getEnv := func(n string, s *string) error {
		if m, ok := nodesInfo[n]; !ok {
			return fmt.Errorf("node %s not found in nodes_info.json", n)
		} else {
			*s = m.Env
		}
		return nil
	}
	if err := getEnv(hostname.Hostname(), &localEnv); err != nil {
		return err
	}
	if err := getEnv(nodename, &peerEnv); err != nil {
		return err
	}
	if localEnv != "PRD" && peerEnv == "PRD" {
		return fmt.Errorf("refuse to sync from a non-PRD node to a PRD node")
	}
	return nil
}

func (t *T) getTargetNodenames(isSourceNode bool) []string {
	if isSourceNode {
		// if the instance is active, check last sync timestamp for each peer
		return t.GetTargetPeernames(t.Target, t.Nodes, t.DRPNodes)
	} else {
		// if the instance is passive, check last sync timestamp for the local node (received from the source node)
		return []string{hostname.Hostname()}
	}
}

// ReplicatesDataset implements ressync.DatasetReplicator: dataset is the one
// sent, or received, or one of their descendants when recursive.
func (t *T) ReplicatesDataset(dataset string) bool {
	for _, root := range []string{t.Src, t.Dst} {
		if dataset == root {
			return true
		}
		if t.Recursive && strings.HasPrefix(dataset, root+"/") {
			return true
		}
	}
	return false
}
