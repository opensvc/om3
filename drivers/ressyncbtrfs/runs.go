package ressyncbtrfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// A sync run takes a read-only snapshot of the source subvolume, and of the
// writable subvolumes nested in it when recursive, in a run directory of its
// own:
//
//	<root of the filesystem>/.osync/<object path>/<rid>/<UTC time of the run>/
//
// and sends them to the peers, each in the run directory of the same name in
// the filesystem of the peer. The peer then replaces its destination
// subvolume by writable snapshots of what it received.
//
// A peer is sent the run incrementally, from the newest run the two hold in
// common, its base. Each peer has its own: a peer that missed runs is sent the
// larger delta at the next one, from the base it holds, while the others go on
// from theirs.
//
// The runs are recognized by the identity of their snapshots, not by their
// names: the uuid of a snapshot of the source is the received uuid of its copy
// on a peer, and a copy sent on keeps it, as btrfs names a parent by its
// received uuid when it has one. So a failover, which makes a peer the
// source, is read as any other run: the new source sends the old one the runs
// it took since, from the newest run they hold in common, the last one it
// received. And so is the failback.
//
// The writes a peer took after the newest run it holds in common with the
// source, as the old source taking writes after its last sync before a
// failover, are lost when the source syncs it: the destination subvolume of
// the peer is replaced by the run it is sent. A peer with its destination
// subvolume mounted is refused, as it runs the service.
//
// The source keeps the base of each peer, so a peer that misses runs holds the
// space its base holds on the source. A peer lagging for longer than
// max_lag_age has its base released, and is not sent anything until an
// administrator asks a full copy for it: the full copy replaces the
// destination subvolume of the peer, which is not a decision to take alone.

type (
	// snap is a read-only snapshot of a run, of the subvolume at Rel from
	// the head subvolume, "" being the head itself.
	snap struct {
		Rel string

		// Path is relative to the root of the filesystem.
		Path string

		// ID is the identity of the snapshot, the same on both ends of a
		// send.
		ID string

		// CGen orders the snapshots of a filesystem by creation.
		CGen int64
	}

	// run is the snapshots a sync run took, in a run directory.
	run struct {
		// Name is the name of the run directory, the UTC time of the run
		// on the node that took it.
		Name  string
		Snaps map[string]snap
	}

	// peerState is what the source remembers of a peer.
	peerState struct {
		// BaseID is the identity of the head snapshot of the run the peer
		// was last sent, which the source keeps for the next incremental
		// send.
		BaseID string `json:"base_id,omitempty"`

		// FailedAt is when a send to the peer first failed since the last
		// one that succeeded. The peer is lagging since.
		FailedAt time.Time `json:"failed_at,omitzero"`

		// StrandedAt is when the source found it could send the peer no
		// increment, for StrandedReason. It takes a full copy asked by an
		// administrator to sync the peer again.
		StrandedAt     time.Time `json:"stranded_at,omitzero"`
		StrandedReason string    `json:"stranded_reason,omitempty"`
	}

	peerStates map[string]peerState

	// syncState is what the source remembers of its peers, in the var dir
	// of the resource.
	syncState struct {
		// LastRunID is the identity of the head snapshot of the run the
		// last sync of this node took. The peers are what this node knew
		// as the source: when the newest run it holds is another one,
		// another node was the source since, and sent it this one, so what
		// it knew of the peers is stale.
		LastRunID string     `json:"last_run_id,omitempty"`
		Peers     peerStates `json:"peers,omitempty"`
	}

	// btrfsOps are the btrfs operations a sync is made of. A nodename
	// names the peer to run them on, "" the local node. A label names the
	// filesystem, whose root they mount when they need it.
	btrfsOps interface {
		// listRuns lists the runs in the directory dir, relative to the
		// root of the filesystem, oldest first.
		listRuns(nodename, label, dir string) ([]run, error)

		// nestedSubvols lists the paths, relative to head, of the
		// writable subvolumes nested in the head subvolume.
		nestedSubvols(label, head string) ([]string, error)

		// takeRun snapshots the head subvolume and the nested ones at
		// rels in the run directory dir.
		takeRun(label, head string, rels []string, dir string) (run, error)

		// send sends the snapshot at path, from parent when not empty, to
		// the run directory dir of the filesystem dstLabel of nodename.
		send(ctx context.Context, nodename, srcLabel, path, parent, dstLabel, dir string) error

		// installHead replaces the head subvolume of nodename by writable
		// snapshots of the run in the directory dir.
		installHead(nodename, label, dir string, r run, head string) error

		// deleteRuns deletes the subvolumes of the run directories in
		// dir, all but the ones named in keep, and the directories.
		deleteRuns(nodename, label, dir string, keep []string) error

		// release unmounts the roots of the filesystems the operations
		// mounted.
		release()
	}
)

const (
	defaultMaxLagAge = 24 * time.Hour

	// headSnapName is the name, in its run directory, of the snapshot of
	// the head subvolume. The snapshot of a nested subvolume is named after
	// its path from the head, escaped.
	headSnapName = "_"

	// runNameFormat is the format of the name of a run directory: the UTC
	// time the run was taken at.
	runNameFormat = "20060102T150405.000000Z"
)

var errStranded = errors.New("stranded")

// snapName is the name of the snapshot of the subvolume at rel from the head,
// in its run directory.
func snapName(rel string) string {
	if rel == "" {
		return headSnapName
	}
	return headSnapName + url.PathEscape(rel)
}

// relOfSnapName is the path from the head of the subvolume a snapshot of the
// name is of.
func relOfSnapName(name string) (string, error) {
	s, ok := strings.CutPrefix(name, headSnapName)
	if !ok {
		return "", fmt.Errorf("%s is not a snapshot name of a run", name)
	}
	return url.PathUnescape(s)
}

// runsDir is the directory of the run directories of the resource, relative to
// the root of the filesystem.
func (t *T) runsDir() string {
	return filepath.Join(".osync", t.Path.String(), t.RID())
}

// newRunName is the name of the run taking its snapshots at now.
func newRunName(now time.Time) string {
	return now.UTC().Format(runNameFormat)
}

func (r run) head() (snap, bool) {
	s, ok := r.Snaps[""]
	return s, ok
}

// headID is the identity of the head snapshot of the run, "" when the run has
// none, as a run a receive was interrupted in.
func (r run) headID() string {
	if s, ok := r.head(); ok {
		return s.ID
	}
	return ""
}

// rels are the paths from the head of the subvolumes of the run, the head
// first and a subvolume after the ones it is nested in.
func (r run) rels() []string {
	l := make([]string, 0, len(r.Snaps))
	for rel := range r.Snaps {
		l = append(l, rel)
	}
	sortRels(l)
	return l
}

// sortRels orders paths so that a path comes after the paths it is below.
func sortRels(l []string) {
	sort.Slice(l, func(i, j int) bool {
		di, dj := strings.Count(l[i], "/"), strings.Count(l[j], "/")
		if l[i] == "" || l[j] == "" {
			return l[i] == ""
		}
		if di != dj {
			return di < dj
		}
		return l[i] < l[j]
	})
}

// sortRuns orders the runs of a node by the creation of their head
// snapshots on it: a run received is ordered by the time it was received,
// whatever the clock of the node that took it.
func sortRuns(l []run) {
	sort.SliceStable(l, func(i, j int) bool {
		hi, _ := l[i].head()
		hj, _ := l[j].head()
		return hi.CGen < hj.CGen
	})
}

// completeRuns are the runs with a head snapshot.
func completeRuns(l []run) []run {
	complete := make([]run, 0, len(l))
	for _, r := range l {
		if r.headID() != "" {
			complete = append(complete, r)
		}
	}
	return complete
}

func findRun(l []run, id string) (run, bool) {
	for _, r := range l {
		if r.headID() == id {
			return r, true
		}
	}
	return run{}, false
}

// newestCommon is the newest run of local the peer holds too.
func newestCommon(local, peer []run) (run, bool) {
	for i := len(local) - 1; i >= 0; i-- {
		if _, ok := findRun(peer, local[i].headID()); ok {
			return local[i], true
		}
	}
	return run{}, false
}

// parentOf is the snapshot of base a snapshot of the subvolume at rel is sent
// from, "" when it is sent whole: a nested subvolume new since the base, or
// one the peer does not hold the same snapshot of, has none.
func parentOf(base, peerBase run, rel string) string {
	s, ok := base.Snaps[rel]
	if !ok {
		return ""
	}
	p, ok := peerBase.Snaps[rel]
	if !ok || p.ID != s.ID {
		return ""
	}
	return s.Path
}

func (t *T) peerStatesFile() string {
	return filepath.Join(t.VarDir(), "peers.json")
}

func (t *T) loadSyncState() (syncState, error) {
	var state syncState
	b, err := os.ReadFile(t.peerStatesFile())
	if errors.Is(err, os.ErrNotExist) {
		return syncState{Peers: make(peerStates)}, nil
	} else if err != nil {
		return state, err
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return state, fmt.Errorf("%s: %w", t.peerStatesFile(), err)
	}
	if state.Peers == nil {
		state.Peers = make(peerStates)
	}
	return state, nil
}

// validPeers is the peers of state, or none when they are stale: newest is
// the identity of the newest run this node holds before the run, "" when it
// holds none.
func (state syncState) validPeers(newest string) peerStates {
	if newest == "" || newest != state.LastRunID {
		return make(peerStates)
	}
	return state.Peers
}

func (t *T) saveSyncState(state syncState) error {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	p := t.peerStatesFile()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (st *peerState) succeeded(id string) {
	*st = peerState{BaseID: id}
}

func (st *peerState) failed(now time.Time) {
	if st.FailedAt.IsZero() {
		st.FailedAt = now
	}
}

func (st *peerState) strand(now time.Time, reason string) {
	st.BaseID = ""
	st.StrandedAt = now
	st.StrandedReason = reason
}

// fullCommand is the command an administrator runs to sync nodename again.
func (t *T) fullCommand(nodename string) string {
	return fmt.Sprintf("om %s instance full --rid %s --target %s", t.Path, t.RID(), nodename)
}

func (t *T) strandedError(nodename string, st peerState) error {
	return fmt.Errorf("%s: %w: %s: run '%s'", nodename, errStranded, st.StrandedReason, t.fullCommand(nodename))
}

func (t *T) maxLagAge() time.Duration {
	if t.MaxLagAge != nil {
		return *t.MaxLagAge
	}
	return defaultMaxLagAge
}

// lagExcess says why a lagging peer can not keep its base any longer, or ""
// when it can.
func (t *T) lagExcess(now time.Time, st peerState) string {
	if st.FailedAt.IsZero() || st.BaseID == "" {
		return ""
	}
	if lag := now.Sub(st.FailedAt); lag > t.maxLagAge() {
		return fmt.Sprintf("lagging for %s, more than max_lag_age %s", lag.Round(time.Second), t.maxLagAge())
	}
	return ""
}

// syncPeer sends r, the run this sync took, to nodename, and updates its
// state. local are the runs of this node, r included.
func (t *T) syncPeer(ctx context.Context, mode modeT, nodename string, local []run, r run, st *peerState, now time.Time) error {
	if mode == modeFull {
		return t.syncPeerFull(ctx, nodename, r, st, now)
	}
	if !st.StrandedAt.IsZero() {
		return t.strandedError(nodename, *st)
	}
	if reason := t.lagExcess(now, *st); reason != "" {
		st.strand(now, reason)
		return t.strandedError(nodename, *st)
	}
	l, err := t.ops.listRuns(nodename, t.dst.label, t.runsDir())
	if err != nil {
		st.failed(now)
		return fmt.Errorf("%s: list runs: %w", nodename, err)
	}
	peer := completeRuns(l)
	if len(peer) == 0 {
		// Never synced: the initial copy is sent without asking, as it
		// always was.
		t.Log().Infof("%s holds no run of %s: send full", nodename, t.RID())
		return t.syncPeerFull(ctx, nodename, r, st, now)
	}
	if received, ok := findRun(peer, r.headID()); ok && len(received.Snaps) == len(r.Snaps) {
		// Sent by a sync that stopped before it installed the copy or
		// saved the state.
		return t.installPeer(nodename, r, st)
	}
	previous := local[:len(local)-1]
	base, ok := newestCommon(previous, peer)
	if !ok {
		st.strand(now, "no run in common with the source")
		return t.strandedError(nodename, *st)
	}
	peerBase, _ := findRun(peer, base.headID())
	for _, rel := range r.rels() {
		parent := parentOf(base, peerBase, rel)
		if err := t.ops.send(ctx, nodename, t.src.label, r.Snaps[rel].Path, parent, t.dst.label, filepath.Join(t.runsDir(), r.Name)); err != nil {
			st.failed(now)
			return fmt.Errorf("%s: send %s: %w", nodename, r.Snaps[rel].Path, err)
		}
	}
	return t.installPeer(nodename, r, st)
}

// syncPeerFull sends r to nodename whole.
func (t *T) syncPeerFull(ctx context.Context, nodename string, r run, st *peerState, now time.Time) error {
	for _, rel := range r.rels() {
		if err := t.ops.send(ctx, nodename, t.src.label, r.Snaps[rel].Path, "", t.dst.label, filepath.Join(t.runsDir(), r.Name)); err != nil {
			st.failed(now)
			return fmt.Errorf("%s: send %s: %w", nodename, r.Snaps[rel].Path, err)
		}
	}
	return t.installPeer(nodename, r, st)
}

// installPeer replaces the destination subvolume of nodename by the run it
// received, and deletes its other runs, which the source no longer sends from.
//
// A failure to install leaves the peer with the run, and with the base it was
// sent from: the next sync installs it, or sends the next run from either.
func (t *T) installPeer(nodename string, r run, st *peerState) error {
	if err := t.ops.installHead(nodename, t.dst.label, filepath.Join(t.runsDir(), r.Name), r, t.dst.subvol); err != nil {
		return fmt.Errorf("%s: install %s: %w", nodename, t.dst.subvol, err)
	}
	st.succeeded(r.headID())
	if err := t.ops.deleteRuns(nodename, t.dst.label, t.runsDir(), []string{r.Name}); err != nil {
		return fmt.Errorf("%s: delete older runs: %w", nodename, err)
	}
	return nil
}

// pruneLocal deletes the runs of the source no peer needs: the newest is
// kept, for the peers up to date, and the base of each peer.
func (t *T) pruneLocal(local []run, peers []string, states peerStates) error {
	if len(local) == 0 {
		return nil
	}
	keep := []string{local[len(local)-1].Name}
	for _, nodename := range peers {
		st := states[nodename]
		if st.BaseID == "" {
			continue
		}
		if r, ok := findRun(local, st.BaseID); ok && !slices.Contains(keep, r.Name) {
			keep = append(keep, r.Name)
		}
	}
	return t.ops.deleteRuns("", t.src.label, t.runsDir(), keep)
}
