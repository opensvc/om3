// Package ressyncbtrfs is the sync.btrfs driver: it replicates a btrfs
// subvolume to the peers with btrfs send and receive. See runs.go for how the
// snapshots are managed across failovers.
package ressyncbtrfs

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/nodesinfo"
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

		src, dst location
		ops      btrfsOps
		conns    sshClients
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

func (t *T) lockAndSync(ctx context.Context, mode modeT) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	target := actioncontext.Target(ctx)
	cancel, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer cancel()
	if t.Timeout != nil && *t.Timeout > 0 {
		var cancelTimeout context.CancelFunc
		ctx, cancelTimeout = context.WithTimeout(ctx, *t.Timeout)
		defer cancelTimeout()
	}
	return t.lockedSync(ctx, mode, target)
}

func (t *T) lockedSync(ctx context.Context, mode modeT, target []string) error {
	isCron := actioncontext.IsCron(ctx)

	if err := t.parse(); err != nil {
		return err
	}
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
	for _, nodename := range nodenames {
		if t.isLocal(nodename) && t.src == t.dst {
			return fmt.Errorf("target local syncs %s:%s to itself: set a dst of another subvolume or filesystem", t.src.label, t.src.subvol)
		}
	}
	done, err := t.StartRun()
	if err != nil {
		return err
	}
	defer done()
	defer t.ops.release()
	defer t.closeSSHClients()

	state, err := t.loadSyncState()
	if err != nil {
		return err
	}
	l, err := t.ops.listRuns("", t.src.label, t.runsDir())
	if err != nil {
		return err
	}
	previous := completeRuns(l)
	var newest string
	if len(previous) > 0 {
		newest = previous[len(previous)-1].headID()
	}
	states := state.validPeers(newest)
	if len(state.Peers) > 0 && len(states) == 0 {
		t.Log().Infof("another node was the source since the last run of this one: forget what it knew of the peers")
	}

	var rels []string
	if t.Recursive {
		if rels, err = t.ops.nestedSubvols(t.src.label, t.src.subvol); err != nil {
			return err
		}
	}
	now := time.Now()
	r, err := t.ops.takeRun(t.src.label, t.src.subvol, rels, filepath.Join(t.runsDir(), newRunName(now)))
	if err != nil {
		return err
	}
	local := append(previous, r)

	// The peers of the configuration, not only the ones of this sync: the
	// base of a peer not synced this time is kept too.
	peers := t.GetTargetPeernames(t.Target, t.Nodes, t.DRPNodes)

	// The peers are synced at once, each on its own: one failing, or slow,
	// does not hold the others back, and all the failures are reported.
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
		nodenames = []string{hostname.Hostname()}
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
