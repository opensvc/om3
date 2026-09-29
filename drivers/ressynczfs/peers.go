package ressynczfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/sizeconv"
)

// A peer is sent the snapshots of the source incrementally, from the newest
// snapshot the two hold in common, its base. Each peer has its own: a peer
// that missed a run is sent the larger delta at the next one, from the base
// it holds, while the others go on from theirs.
//
// The snapshots are recognized by their guid, not their name: a snapshot a
// peer received is the one of the source with the same guid, whatever either
// end calls it. So a failover, which makes the receiver the source, an
// interrupted run, and the names older agents gave their snapshots are all
// read the same way.
//
// The source keeps the base of each peer, so a peer that misses runs holds the
// space its base holds on the source. A peer lagging for longer than
// max_lag_age, or holding more than max_lag_size, has its base released, and
// is not sent anything until an administrator asks a full copy for it: the
// full copy replaces the dataset of the peer, which is not a decision to take
// alone.

type (
	snapshot struct {
		// Name is the full name, <dataset>@<name>.
		Name string

		// GUID is the same on both ends of a send.
		GUID string

		// CreateTxg orders the snapshots of a dataset.
		CreateTxg uint64
	}

	// peerState is what the source remembers of a peer.
	peerState struct {
		// BaseGUID is the guid of the snapshot the peer was last sent,
		// which the source keeps for the next incremental send.
		BaseGUID string `json:"base_guid,omitempty"`

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

	// zfsOps are the zfs operations a sync is made of. A nodename names
	// the peer to run them on, the local node being named too when it is
	// a target of the sync.
	zfsOps interface {
		listSnapshots(nodename, dataset string) ([]snapshot, error)
		takeSnapshot(name string) error
		destroySnapshot(nodename, name string) error

		// reclaimable is the space destroying the snapshots of dataset
		// from first to last, both included, would free.
		reclaimable(dataset, first, last string) (int64, error)

		// available is the space the datasets of the pool can still use.
		available(pool string) (int64, error)

		sendFull(ctx context.Context, nodename, snap string) error
		sendIncremental(ctx context.Context, nodename, base, snap string) error
	}
)

const (
	defaultMaxLagAge  = 24 * time.Hour
	defaultMaxLagSize = "20%"
)

var errStranded = errors.New("stranded")

// snapPrefix is the start of the names of the snapshots of this resource.
func (t *T) snapPrefix() string {
	return strings.Replace(t.RID(), "#", ".", 1) + "."
}

// isOwnSnapshot reports whether name, a <dataset>@<name>, is a snapshot of
// this resource.
func (t *T) isOwnSnapshot(name string) bool {
	_, snap, ok := strings.Cut(name, "@")
	return ok && strings.HasPrefix(snap, t.snapPrefix())
}

// isLegacySnapshot reports whether name is one of the two snapshots older
// agents rotated.
func (t *T) isLegacySnapshot(name string) bool {
	_, snap, _ := strings.Cut(name, "@")
	return snap == t.snapPrefix()+"sent" || snap == t.snapPrefix()+"tosend"
}

// newSnapName is the name of the snapshot a run taking it at now takes.
func (t *T) newSnapName(now time.Time) string {
	return t.Src + "@" + t.snapPrefix() + now.UTC().Format("20060102T150405.000000Z")
}

func (t *T) ownSnapshots(l []snapshot) []snapshot {
	own := make([]snapshot, 0, len(l))
	for _, s := range l {
		if t.isOwnSnapshot(s.Name) {
			own = append(own, s)
		}
	}
	slices.SortFunc(own, func(a, b snapshot) int {
		switch {
		case a.CreateTxg < b.CreateTxg:
			return -1
		case a.CreateTxg > b.CreateTxg:
			return 1
		default:
			return 0
		}
	})
	return own
}

func findGUID(l []snapshot, guid string) (snapshot, bool) {
	for _, s := range l {
		if s.GUID == guid {
			return s, true
		}
	}
	return snapshot{}, false
}

// newestCommon is the newest snapshot of local the peer holds too.
func newestCommon(local, peer []snapshot) (snapshot, bool) {
	for i := len(local) - 1; i >= 0; i-- {
		if _, ok := findGUID(peer, local[i].GUID); ok {
			return local[i], true
		}
	}
	return snapshot{}, false
}

func snapShortName(name string) string {
	_, snap, _ := strings.Cut(name, "@")
	return snap
}

func (t *T) peerStatesFile() string {
	return filepath.Join(t.VarDir(), "peers.json")
}

// syncState is what the source remembers of its peers, in the var dir of the
// resource.
type syncState struct {
	// LastSnapGUID is the guid of the snapshot the last run took. The
	// peers are what this node knew as the source: when the newest
	// snapshot it holds is another one, another node was the source since,
	// and sent it this one, so what it knew of the peers is stale.
	LastSnapGUID string     `json:"last_snap_guid,omitempty"`
	Peers        peerStates `json:"peers,omitempty"`
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
// the newest snapshot of the resource this node holds before the run, nil
// when it holds none.
func (state syncState) validPeers(newest *snapshot) peerStates {
	if newest == nil || newest.GUID != state.LastSnapGUID {
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

func (st *peerState) succeeded(guid string) {
	*st = peerState{BaseGUID: guid}
}

func (st *peerState) failed(now time.Time) {
	if st.FailedAt.IsZero() {
		st.FailedAt = now
	}
}

func (st *peerState) strand(now time.Time, reason string) {
	st.BaseGUID = ""
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

// maxLagSizeOf is the space a lagging peer may hold, held being what it holds
// now: a percentage is of the space the pool would have free without it.
func (t *T) maxLagSizeOf(held int64) (int64, error) {
	s := t.MaxLagSize
	if s == "" {
		s = defaultMaxLagSize
	}
	pct, isPct, err := parseMaxLagSize(s)
	if err != nil {
		return 0, err
	}
	if !isPct {
		return int64(pct), nil
	}
	avail, err := t.ops.available(t.zfs(t.Src).PoolName())
	if err != nil {
		return 0, err
	}
	return int64(pct / 100 * float64(avail+held)), nil
}

// parseMaxLagSize reads a max_lag_size value: a size, or a percentage when
// it ends with %.
func parseMaxLagSize(s string) (v float64, isPct bool, err error) {
	if p, ok := strings.CutSuffix(s, "%"); ok {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || v < 0 || v > 100 {
			return 0, true, fmt.Errorf("%s is not a percentage", s)
		}
		return v, true, nil
	}
	n, err := sizeconv.FromSize(s)
	if err != nil {
		return 0, false, err
	}
	if n < 0 {
		return 0, false, fmt.Errorf("%s is not a size", s)
	}
	return float64(n), false, nil
}

func validateMaxLagSize(s string) error {
	_, _, err := parseMaxLagSize(s)
	return err
}

// lagExcess says why a lagging peer can not keep its base any longer, or ""
// when it can.
func (t *T) lagExcess(now time.Time, st peerState, local []snapshot) (string, error) {
	if st.FailedAt.IsZero() || st.BaseGUID == "" {
		return "", nil
	}
	if lag := now.Sub(st.FailedAt); lag > t.maxLagAge() {
		return fmt.Sprintf("lagging for %s, more than max_lag_age %s", lag.Round(time.Second), t.maxLagAge()), nil
	}
	base, ok := findGUID(local, st.BaseGUID)
	if !ok {
		return "", nil
	}
	// The snapshots from the base of the peer to the one before the
	// newest are what the source keeps for it, the newest being kept for
	// the peers up to date. It counts the churn of the last interval too,
	// which the source releases at the end of a run anyway: the space held
	// is overstated by that much, never understated.
	newest := len(local) - 1
	if local[newest].GUID == base.GUID {
		return "", nil
	}
	last := local[newest-1]
	held, err := t.ops.reclaimable(t.Src, snapShortName(base.Name), snapShortName(last.Name))
	if err != nil {
		return "", err
	}
	limit, err := t.maxLagSizeOf(held)
	if err != nil {
		return "", err
	}
	if held > limit {
		return fmt.Sprintf("holding %s on the source, more than max_lag_size %s",
			sizeconv.BSizeCompact(float64(held)), sizeconv.BSizeCompact(float64(limit))), nil
	}
	return "", nil
}

// syncPeer sends snap, the snapshot this run took, to nodename, and updates
// its state.
func (t *T) syncPeer(ctx context.Context, mode modeT, nodename string, local []snapshot, snap snapshot, st *peerState, now time.Time) error {
	if mode == modeFull {
		return t.syncPeerFull(ctx, nodename, local, snap, st, now)
	}
	if !st.StrandedAt.IsZero() {
		return t.strandedError(nodename, *st)
	}
	if reason, err := t.lagExcess(now, *st, local); err != nil {
		return fmt.Errorf("%s: lag: %w", nodename, err)
	} else if reason != "" {
		st.strand(now, reason)
		return t.strandedError(nodename, *st)
	}
	l, err := t.ops.listSnapshots(nodename, t.Dst)
	if err != nil {
		st.failed(now)
		return fmt.Errorf("%s: list snapshots: %w", nodename, err)
	}
	peer := t.ownSnapshots(l)
	if len(peer) == 0 {
		// Never synced: the initial copy is sent without asking, as it
		// always was.
		t.Log().Infof("%s holds no snapshot of %s: send full", nodename, t.RID())
		return t.syncPeerFull(ctx, nodename, local, snap, st, now)
	}
	if _, ok := findGUID(peer, snap.GUID); ok {
		// Sent by a run that stopped before it saved the state.
		st.succeeded(snap.GUID)
		return nil
	}
	previous := local[:len(local)-1]
	base, ok := newestCommon(previous, peer)
	if !ok {
		st.strand(now, "no snapshot in common with the source")
		return t.strandedError(nodename, *st)
	}
	if err := t.ops.sendIncremental(ctx, nodename, base.Name, snap.Name); err != nil {
		st.failed(now)
		return err
	}
	st.succeeded(snap.GUID)

	// The peer holds its older snapshots, and the ones of the source
	// between its base and snap that an intermediary send brought: all
	// but snap go.
	var errs error
	for _, s := range peer {
		if s.GUID != snap.GUID {
			errs = errors.Join(errs, t.ops.destroySnapshot(nodename, t.Dst+"@"+snapShortName(s.Name)))
		}
	}
	if t.Intermediary {
		for _, s := range previous {
			if s.CreateTxg > base.CreateTxg {
				if _, ok := findGUID(peer, s.GUID); !ok {
					errs = errors.Join(errs, t.ops.destroySnapshot(nodename, t.Dst+"@"+snapShortName(s.Name)))
				}
			}
		}
	}
	if errs != nil {
		return fmt.Errorf("%s: destroy older snapshots: %w", nodename, errs)
	}
	return nil
}

// seedLegacyStates gives the peers with no state yet the base older agents
// synced them from: those agents rotated two snapshots, the base of every peer
// being the "sent" one, or the "tosend" one when a run failed half way.
//
// Without it, a peer out of reach on the first run of this agent would have
// its base pruned, and need a full copy once back.
func (t *T) seedLegacyStates(local []snapshot, peers []string, states peerStates) {
	var legacy snapshot
	for _, s := range local {
		if t.isLegacySnapshot(s.Name) && (legacy.Name == "" || strings.HasSuffix(s.Name, ".sent")) {
			legacy = s
		}
	}
	if legacy.Name == "" {
		return
	}
	for _, nodename := range peers {
		if _, ok := states[nodename]; !ok {
			states[nodename] = peerState{BaseGUID: legacy.GUID}
		}
	}
}

// syncPeerFull sends snap to nodename whole, replacing its dataset.
//
// A recursive send carries the older snapshots of the source too, which the
// peer needs none of.
func (t *T) syncPeerFull(ctx context.Context, nodename string, local []snapshot, snap snapshot, st *peerState, now time.Time) error {
	if err := t.ops.sendFull(ctx, nodename, snap.Name); err != nil {
		st.failed(now)
		return err
	}
	st.succeeded(snap.GUID)
	if !t.Recursive {
		return nil
	}
	var errs error
	for _, s := range local {
		if s.GUID != snap.GUID {
			errs = errors.Join(errs, t.ops.destroySnapshot(nodename, t.Dst+"@"+snapShortName(s.Name)))
		}
	}
	if errs != nil {
		return fmt.Errorf("%s: destroy older snapshots: %w", nodename, errs)
	}
	return nil
}

// pruneLocal destroys the snapshots of the source no peer needs: the newest
// is kept, for the peers up to date, and the base of each peer.
//
// The two snapshots older agents rotated are kept together while one of them
// is the base of a peer, which may have received either.
func (t *T) pruneLocal(local []snapshot, peers []string, states peerStates) error {
	if len(local) == 0 {
		return nil
	}
	keep := map[string]bool{local[len(local)-1].GUID: true}
	for _, nodename := range peers {
		if st := states[nodename]; st.BaseGUID != "" {
			keep[st.BaseGUID] = true
		}
	}
	keepLegacy := false
	for _, s := range local {
		if keep[s.GUID] && t.isLegacySnapshot(s.Name) {
			keepLegacy = true
		}
	}
	var errs error
	for _, s := range local {
		if keep[s.GUID] || (keepLegacy && t.isLegacySnapshot(s.Name)) {
			continue
		}
		errs = errors.Join(errs, t.ops.destroySnapshot("", s.Name))
	}
	return errs
}
