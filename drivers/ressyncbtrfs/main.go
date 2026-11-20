// Package ressyncbtrfs is the sync.btrfs driver: it replicates a btrfs
// subvolume to the peers with btrfs send and receive. See runs.go for how the
// snapshots are managed across failovers.
package ressyncbtrfs

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/drivers/ressync"
	"github.com/opensvc/om3/v3/util/btrfs"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	// T is the driver structure.
	T struct {
		ressync.T
		resource.SSH
		Src       string
		Dst       string
		Target    []string
		Recursive bool
		Nodes     []string
		DRPNodes  []string
		ObjectID  uuid.UUID
		Timeout   *time.Duration
		Topology  topology.T
		MaxLagAge *time.Duration
		DelaySnap bool

		src, dst location
		ops      btrfsOps
		conns    sshClients

		// prepared is the run PrepareSync took for the transfer of the
		// action, nil when it took none: the transfer sends it, the lock
		// and the run file held until its release.
		prepared *preparedSync
	}

	// preparedSync is the run PrepareSync took.
	preparedSync struct {
		// run is the snapshots taken.
		run run

		// takenAt is when they were taken, the time of the sync sending
		// them.
		takenAt time.Time

		// sent says a sync sends the run to the peers: it is then the
		// base of the next sync, and no longer the release's to delete.
		sent bool
	}

	// location is a subvolume of the btrfs filesystem of a label, as the
	// src and dst keywords name it: <label>:<subvol>.
	location struct {
		label  string
		subvol string
	}

	modeT uint
)

const (
	modeFull modeT = iota
	modeIncr

	lockName = "sync"
)

// SyncsTo implements resource.PeerSyncer.
func (t *T) SyncsTo(nodename string) bool {
	return slices.Contains(t.GetTargetPeernames(t.Target, t.Nodes, t.DRPNodes), nodename)
}

// SyncSource implements resource.PeerSyncer.
func (t *T) SyncSource() string {
	return t.Src
}

func New() resource.Driver {
	return &T{}
}

// parseLocation reads a <label>:<subvol> keyword value.
//
// The subvolume is not the directory the snapshots of the syncs are kept in,
// nor the one a sync stages a copy in: the destination is replaced at every
// sync, so a value naming more than a subvolume of its own is refused.
func parseLocation(s string) (location, error) {
	l, err := btrfs.ParseLocation(s)
	if err != nil {
		return location{}, err
	}
	switch {
	case l.Subvol == ".osync" || strings.HasPrefix(l.Subvol, ".osync/"):
		return location{}, fmt.Errorf("%q names the directory the snapshots of the syncs are kept in", s)
	case strings.HasSuffix(l.Subvol, ".osync-new"):
		return location{}, fmt.Errorf("%q names the subvolume a sync stages a copy in", s)
	}
	return location{label: l.Label, subvol: l.Subvol}, nil
}

func (t *T) Configure() error {
	if t.ops == nil {
		t.ops = &execOps{t: t}
	}
	return nil
}

// parse reads the src and dst keywords, dst defaulting to src.
func (t *T) parse() error {
	src, err := parseLocation(t.Src)
	if err != nil {
		return fmt.Errorf("src: %w", err)
	}
	dst := src
	if t.Dst != "" {
		if dst, err = parseLocation(t.Dst); err != nil {
			return fmt.Errorf("dst: %w", err)
		}
	}
	t.src, t.dst = src, dst
	return nil
}

func (t *T) Full(ctx context.Context) error {
	return t.lockAndSync(ctx, modeFull)
}

func (t *T) Update(ctx context.Context) error {
	return t.lockAndSync(ctx, modeIncr)
}

// lockAndSync runs an update or a full, under the lock of the resource, which
// a PrepareSync of the action holds already.
func (t *T) lockAndSync(ctx context.Context, mode modeT) error {
	target := actioncontext.Target(ctx)
	if t.prepared == nil {
		disable := actioncontext.IsLockDisabled(ctx)
		timeout := actioncontext.LockTimeout(ctx)
		cancel, err := t.Lock(disable, timeout, lockName)
		if err != nil {
			return err
		}
		defer cancel()
	}
	if t.Timeout != nil && *t.Timeout > 0 {
		var cancelTimeout context.CancelFunc
		ctx, cancelTimeout = context.WithTimeout(ctx, *t.Timeout)
		defer cancelTimeout()
	}
	return t.lockedSync(ctx, mode, target)
}

// plan returns the peers an update or a full sends to, none when it sends
// nothing, as the environment, or no peer to send to, says, logged unless
// quiet. An error is a refusal, as a flex instance not primary. It changes
// nothing but the locations read from the keywords, so it also says whether
// a sync would take a run.
func (t *T) plan(ctx context.Context, target []string, quiet bool) ([]string, error) {
	if err := t.parse(); err != nil {
		return nil, err
	}
	if t.isFlexAndNotPrimary() {
		return nil, fmt.Errorf("this flex instance is not primary. only %s can sync", t.Nodes[0])
	}
	if reason := t.EnvSkipReason(); reason != "" {
		switch {
		case quiet:
		case actioncontext.IsCron(ctx):
			t.Log().Tracef("%s", reason)
		default:
			t.Log().Infof("%s", reason)
		}
		return nil, nil
	}
	if v, reason := t.IsInstanceSufficientlyStarted(ctx); !v {
		return nil, fmt.Errorf("the instance is not sufficiently started (%s). refuse to sync to protect the data of the started remote instance", reason)
	}
	nodenames, err := t.SelectPeernames(target, t.Target, t.Nodes, t.DRPNodes)
	if err != nil {
		return nil, err
	}
	if len(nodenames) == 0 {
		if !quiet {
			t.Log().Infof("no peer to sync")
		}
		return nil, nil
	}
	for _, nodename := range nodenames {
		if t.isLocal(nodename) && t.src == t.dst {
			return nil, fmt.Errorf("target local syncs %s:%s to itself: set a dst of another subvolume or filesystem", t.src.label, t.src.subvol)
		}
	}
	return nodenames, nil
}

// CopiesSyncSnapshot implements resource.SyncSnapshotCopier: the run a sync
// sends is taken with the snapshots of the other sync resources of the
// action, unless delay_snap has it taken just before its own transfer.
func (t *T) CopiesSyncSnapshot() bool {
	return !t.DelaySnap
}

// WillPrepareSync implements resource.SyncPreparer: the run a sync sends is
// taken before the transfers of the action, unless delay_snap has it taken
// just before its own, or the sync would send nothing.
func (t *T) WillPrepareSync(ctx context.Context) bool {
	if !t.CopiesSyncSnapshot() {
		return false
	}
	nodenames, err := t.plan(ctx, actioncontext.Target(ctx), true)
	return err == nil && len(nodenames) > 0
}

// PrepareSync implements resource.SyncPreparer: it takes the run the
// transfer of the action sends, under the lock of the resource, held with
// the run file until the release. A concurrent update would take a newer
// run, and delete this one as a run no peer needs.
//
// The run a sync sent is the base of the next: the release keeps it. One no
// sync sent, as when the action failed before the transfer of the resource,
// is deleted: left as the newest run of the resource, the next sync would
// read it as received from another source, and forget what this node knows
// of its peers.
//
// The root of the filesystem the run is taken in stays mounted for the
// transfer, which unmounts it, or the release.
func (t *T) PrepareSync(ctx context.Context) (func() error, error) {
	unlock, err := t.Lock(actioncontext.IsLockDisabled(ctx), actioncontext.LockTimeout(ctx), lockName)
	if err != nil {
		return nil, err
	}
	done, err := t.StartRun()
	if err != nil {
		unlock()
		return nil, err
	}
	release := func() error {
		prepared := t.prepared
		t.prepared = nil
		defer unlock()
		defer done()
		defer t.ops.release()
		if prepared == nil || prepared.sent {
			return nil
		}
		if err := t.deleteRun(prepared.run); err != nil {
			return fmt.Errorf("delete the run %s no sync sent: %w", prepared.run.Name, err)
		}
		return nil
	}
	nodenames, err := t.plan(ctx, actioncontext.Target(ctx), true)
	if err != nil {
		return nil, errors.Join(err, release())
	}
	if len(nodenames) == 0 {
		return nil, release()
	}
	r, now, err := t.takeRun()
	if err != nil {
		return nil, errors.Join(err, release())
	}
	t.prepared = &preparedSync{run: r, takenAt: now}
	return release, nil
}

// takeRun takes a run of the source subvolume now, and of the writable
// subvolumes nested in it when recursive.
func (t *T) takeRun() (run, time.Time, error) {
	var (
		rels []string
		err  error
	)
	if t.Recursive {
		if rels, err = t.ops.nestedSubvols(t.src.label, t.src.subvol); err != nil {
			return run{}, time.Time{}, err
		}
	}
	now := time.Now()
	r, err := t.ops.takeRun(t.src.label, t.src.subvol, rels, filepath.Join(t.runsDir(), newRunName(now)))
	return r, now, err
}

// deleteRun deletes the run r of the source, and no other.
func (t *T) deleteRun(r run) error {
	l, err := t.ops.listRuns("", t.src.label, t.runsDir())
	if err != nil {
		return err
	}
	keep := make([]string, 0, len(l))
	for _, other := range l {
		if other.Name != r.Name {
			keep = append(keep, other.Name)
		}
	}
	return t.ops.deleteRuns("", t.src.label, t.runsDir(), keep)
}

// syncRun returns the run a sync sends, the runs the source held before it,
// oldest first, and the time of the sync: the run the action took before its
// first transfer, or one taken now.
func (t *T) syncRun() (run, []run, time.Time, error) {
	l, err := t.ops.listRuns("", t.src.label, t.runsDir())
	if err != nil {
		return run{}, nil, time.Time{}, err
	}
	previous := completeRuns(l)
	if t.prepared != nil {
		previous = slices.DeleteFunc(previous, func(r run) bool { return r.Name == t.prepared.run.Name })
		// A peer may hold it from here on.
		t.prepared.sent = true
		return t.prepared.run, previous, t.prepared.takenAt, nil
	}
	r, now, err := t.takeRun()
	return r, previous, now, err
}

func (t *T) lockedSync(ctx context.Context, mode modeT, target []string) error {
	isCron := actioncontext.IsCron(ctx)

	nodenames, err := t.plan(ctx, target, false)
	if err != nil || len(nodenames) == 0 {
		return err
	}
	if t.prepared == nil {
		// The run file of a PrepareSync is held until its release.
		done, err := t.StartRun()
		if err != nil {
			return err
		}
		defer done()
	}
	defer t.ops.release()
	defer t.closeSSHClients()

	state, err := t.loadSyncState()
	if err != nil {
		return err
	}
	r, previous, now, err := t.syncRun()
	if err != nil {
		return err
	}
	var newest string
	if len(previous) > 0 {
		newest = previous[len(previous)-1].headID()
	}
	states := state.validPeers(newest)
	switch {
	case len(state.Peers) == 0 || len(states) > 0:
	case newest == "":
		t.Log().Infof("the filesystem holds no run of this resource, as a new one: forget what it knew of the peers")
	default:
		t.Log().Infof("another node was the source since the last run of this one: forget what it knew of the peers")
	}

	local := append(previous, r)

	// The peers of the configuration, not only the ones of this sync: the
	// base of a peer not synced this time is kept too.
	peers := t.GetTargetPeernames(t.Target, t.Nodes, t.DRPNodes)
	t.seedReceivedBase(local, peers, states)

	// The peers are synced at once, each on its own: one failing, or slow,
	// does not hold the others back, and all the failures are reported.
	var (
		wg       sync.WaitGroup
		peerErrs = make([]error, len(nodenames))
		results  = make([]*peerState, len(nodenames))
	)
	for i, nodename := range nodenames {
		if err := t.PeerEnvRefusal(nodename); err != nil {
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
			if err := t.syncPeer(ctx, mode, nodename, local, r, &st, now); err != nil {
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
	// The runs of the peers synced but the one sent go once every peer is
	// synced, but the bases of the others as the sync leaves them.
	for i, nodename := range nodenames {
		if results[i] != nil && peerErrs[i] == nil {
			peerErrs[i] = t.prunePeer(nodename, local, r, otherBases(states, nodename))
		}
	}
	errs := errors.Join(peerErrs...)
	if err := t.saveSyncState(syncState{LastRunID: r.headID(), Peers: states}); err != nil {
		return errors.Join(errs, err)
	}
	if err := t.pruneLocal(local, peers, states); err != nil {
		errs = errors.Join(errs, fmt.Errorf("delete the runs no peer needs: %w", err))
	}
	return errs
}

func (t *T) Kill(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	isSourceNode := true
	if v, _ := t.IsInstanceSufficientlyStarted(ctx); !v {
		isSourceNode = false
	} else if t.isFlexAndNotPrimary() {
		isSourceNode = false
	}
	var nodenames []string
	if isSourceNode {
		nodenames = t.GetTargetPeernames(t.Target, t.Nodes, t.DRPNodes)
	} else {
		nodenames = t.ReceivingNodenames(t.Target, t.Nodes, t.DRPNodes)
	}
	state := t.StatusLastSync(nodenames, !isSourceNode)
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
	result := status.Undef
	for _, nodename := range nodenames {
		st, ok := state.Peers[nodename]
		if !ok || st.StrandedAt.IsZero() {
			continue
		}
		t.StatusLog().Warn("%s: not synced since %s: %s: run '%s'", nodename, st.StrandedAt.Format(time.RFC3339), st.StrandedReason, t.fullCommand(nodename))
		result.Add(status.Warn)
	}
	return result
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

func (t *T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action:                   "update",
		Option:                   "schedule",
		Base:                     "",
		RequireReplicationSource: true,
		Require:                  t.UpdateRequires,
		Log:                      t.LogRuns,
	}
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
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
		m = append(m, resource.InfoKey{Key: "timeout", Value: t.Timeout.String()})
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
