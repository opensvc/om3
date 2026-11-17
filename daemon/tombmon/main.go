// Package tombmon keeps the tombstone records of the node: it loads them at
// start, merges the copies its peers publish, persists and publishes its
// own, clears the holders evicted, demotes to away the live holders seen
// away, and removes a record once nothing needs it any more.
//
// The records are written by the imon of an object deleting its instance
// with scope nodes it did not see run the orchestration (Record), cleared by
// the holders themselves (ClearLocal), and dropped by a create --force of
// the same name (Drop).
package tombmon

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	// Manager keeps the tombstone records of the node.
	Manager struct {
		ctx    context.Context
		cancel context.CancelFunc
		log    *plog.Logger
		subQS  pubsub.QueueSizer
		wg     sync.WaitGroup
	}

	// state is what the package API and the manager share, under mu.
	state struct {
		dir       string
		localhost string
		publisher pubsub.Publisher
		log       *plog.Logger

		// records are the records the node keeps, by orchestration id:
		// the ones of its files.
		records map[string]tombstone.Record

		// baseline is the modification time of the config file of the
		// away holder of the node, by record id, see evaluateLocal.
		baseline map[string]time.Time

		// byPath indexes the ids of the records by path: an event about
		// an object looks at its records only.
		byPath map[string]map[string]bool

		// dirty and removed are the records to write and the files to
		// remove at the next flush, see flush.
		dirty   map[string]bool
		removed map[string]bool

		// nodeAway is whether each node was last seen away: a node
		// monitor update changes nothing unless it changes this.
		nodeAway map[string]bool

		// rejoined is whether the node monitor of the node was seen out
		// of its start and rejoin.
		rejoined bool

		// gone holds the records the node removed, by id, for
		// removedMemory: a copy of one still in flight from a peer is
		// stale, and adopted it would be removed again, the peers doing
		// the same with the copies of the node, round after round.
		gone map[string]goneRecord

		// outbox holds the messages to publish, in order, by the
		// publisher, out of mu: a publication waits for the bus, which
		// would hold up every caller of mu. queued and published count
		// the messages put in it and the ones published, for a caller to
		// wait for its own.
		outbox    []outMsg
		queued    uint64
		published uint64
		stopped   bool
		cond      *sync.Cond
	}

	// goneRecord is what the node remembers of a record it removed: when,
	// and how its own holder was cleared, empty if it was none.
	goneRecord struct {
		at time.Time
		by tombstone.ClearedBy
	}

	outMsg struct {
		msg    pubsub.Messager
		labels []pubsub.Label
	}
)

// removedMemory is how long a record removed is remembered: a record is
// removed once every node is up and none publishes it pending, so a copy
// of it arriving later is one in flight, never a new one.
var removedMemory = time.Hour

// flushInterval is the pace of the writes of the records changed by the
// events: a merge or a clear is published at once and written in a batch,
// one sync of the directory for all, as a sync per change held up the
// events of a delete of many objects.
var flushInterval = 500 * time.Millisecond

// sweepInterval is the pace of the looks at the away holder of the node and
// of the removal of the records all cleared.
var sweepInterval = 10 * time.Second

var (
	mu sync.Mutex
	st *state

	// flushMu serializes the flushes, so a flush never writes an older
	// copy over the one a later flush wrote.
	flushMu sync.Mutex

	// ErrNotStarted is the error of a call made while the manager is not
	// running.
	ErrNotStarted = errors.New("the tombstone manager is not running")
)

// NewManager returns the manager of the tombstone records of the node.
func NewManager(subQS pubsub.QueueSizer) *Manager {
	return &Manager{
		log:   plog.NewDefaultLogger().Attr("pkg", "daemon/tombmon").WithPrefix("daemon: tombmon: "),
		subQS: subQS,
	}
}

// Start loads the records of the node and publishes them, before the node
// monitor leaves the rejoin: a peer seeing this node up sees its records
// too.
func (t *Manager) Start(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)
	s := &state{
		dir:       tombstone.Dir(),
		localhost: hostname.Hostname(),
		publisher: pubsub.PubFromContext(t.ctx),
		log:       t.log,
		records:   make(map[string]tombstone.Record),
		baseline:  make(map[string]time.Time),
		byPath:    make(map[string]map[string]bool),
		dirty:     make(map[string]bool),
		removed:   make(map[string]bool),
		nodeAway:  make(map[string]bool),
		gone:      make(map[string]goneRecord),
		cond:      sync.NewCond(&mu),
	}
	records, err := tombstone.Load(s.dir)
	if err != nil {
		t.log.Errorf("load %s: %s", s.dir, err)
	}
	mu.Lock()
	st = s
	for _, rec := range records {
		// The daemon restarted: its instance may have been changed
		// while it was down, so it is not trusted to complete the
		// delete on its own.
		if demote(&rec, s.localhost) {
			s.dirty[rec.OrchestrationID] = true
		}
		s.set(rec)
		s.publish(rec)
		t.log.Infof("loaded the tombstone %s of %s, pending holders %s", rec.OrchestrationID, rec.Path, rec.PendingHolders(""))
	}
	mu.Unlock()
	flush()
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		s.publishLoop()
	}()
	// Published before the node monitor starts, as said above.
	s.waitPublished(s.queuedCount())

	sub := t.startSubscriptions()
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer func() {
			if err := sub.Stop(); err != nil {
				t.log.Warnf("subscription stop: %s", err)
			}
		}()
		t.loop(sub)
	}()
	return nil
}

func (t *Manager) Stop() error {
	t.cancel()
	mu.Lock()
	if st != nil {
		st.stopped = true
		st.cond.Broadcast()
	}
	mu.Unlock()
	t.wg.Wait()
	flush()
	mu.Lock()
	st = nil
	mu.Unlock()
	return nil
}

// publishLoop publishes the messages of the outbox, in order, until the
// manager stops, the outbox drained.
func (s *state) publishLoop() {
	for {
		mu.Lock()
		for len(s.outbox) == 0 && !s.stopped {
			s.cond.Wait()
		}
		if len(s.outbox) == 0 {
			mu.Unlock()
			return
		}
		batch := s.outbox
		s.outbox = nil
		mu.Unlock()
		for _, m := range batch {
			s.publisher.Pub(m.msg, m.labels...)
		}
		mu.Lock()
		s.published += uint64(len(batch))
		s.cond.Broadcast()
		mu.Unlock()
	}
}

// enqueue puts a message in the outbox. The caller holds mu.
// Without a publisher, as in the tests, it is published at once.
func (s *state) enqueue(msg pubsub.Messager, labels ...pubsub.Label) {
	s.queued++
	if s.cond == nil {
		s.publisher.Pub(msg, labels...)
		s.published++
		return
	}
	s.outbox = append(s.outbox, outMsg{msg: msg, labels: labels})
	s.cond.Broadcast()
}

func (s *state) queuedCount() uint64 {
	mu.Lock()
	defer mu.Unlock()
	return s.queued
}

// waitPublished returns once the messages queued up to n are published, or
// the manager stops.
func (s *state) waitPublished(n uint64) {
	mu.Lock()
	defer mu.Unlock()
	for s.published < n && !s.stopped && s.cond != nil {
		s.cond.Wait()
	}
}

func (t *Manager) startSubscriptions() *pubsub.Subscription {
	sub := pubsub.SubFromContext(t.ctx, "daemon.tombmon", t.subQS)
	var filters pubsub.Filters
	filters.Add(&msgbus.NodeTombstoneUpdated{})
	filters.Add(&msgbus.NodeTombstoneDeleted{})
	filters.Add(&msgbus.NodeMonitorUpdated{})
	filters.Add(&msgbus.NodeMonitorDeleted{})
	filters.Add(&msgbus.ClusterConfigUpdated{})
	// What the away holder of the node looks at: its config file and its
	// config data, and the sweep for the quiet of its instance. Not every
	// instance monitor update of the node, which a delete of many objects
	// floods.
	localhost := pubsub.Label{"node", hostname.Hostname()}
	filters.Add(&msgbus.ConfigFileUpdated{})
	filters.Add(&msgbus.InstanceConfigDeleted{}, localhost)
	sub.AddFilters(filters)
	sub.Start()
	return sub
}

func (t *Manager) loop(sub *pubsub.Subscription) {
	flushTicker := time.NewTicker(flushInterval)
	defer flushTicker.Stop()
	sweepTicker := time.NewTicker(sweepInterval)
	defer sweepTicker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-sub.C:
			mu.Lock()
			if st != nil {
				st.handle(i)
			}
			mu.Unlock()
		case <-flushTicker.C:
			flush()
		case <-sweepTicker.C:
			mu.Lock()
			if st != nil {
				st.reconcile()
				st.evaluateLocal("")
				st.removeAllIfDone()
				st.forgetGone()
			}
			mu.Unlock()
		}
	}
}

// flush writes the records changed and removes the files of the records
// removed since the last flush, then syncs the directory once. It returns
// the errors by id; a record not written is written, a file not removed is
// removed, at the next flush.
func flush() map[string]error {
	flushMu.Lock()
	defer flushMu.Unlock()
	mu.Lock()
	s := st
	if s == nil {
		mu.Unlock()
		return nil
	}
	writes := make([]tombstone.Record, 0, len(s.dirty))
	for id := range s.dirty {
		if rec, ok := s.records[id]; ok {
			writes = append(writes, rec.DeepCopy())
		}
	}
	removes := make([]string, 0, len(s.removed))
	for id := range s.removed {
		removes = append(removes, id)
	}
	s.dirty = make(map[string]bool)
	s.removed = make(map[string]bool)
	dir, log := s.dir, s.log
	mu.Unlock()

	errs := make(map[string]error)
	for _, rec := range writes {
		if err := tombstone.Write(dir, rec); err != nil {
			errs[rec.OrchestrationID] = err
			log.Errorf("write the tombstone %s of %s: %s", rec.OrchestrationID, rec.Path, err)
		}
	}
	for _, id := range removes {
		if err := tombstone.RemoveFile(dir, id); err != nil {
			errs[id] = err
			log.Errorf("remove the tombstone %s: %s", id, err)
		}
	}
	if len(writes)+len(removes) > 0 {
		if err := tombstone.SyncDir(dir); err != nil {
			log.Errorf("sync %s: %s", dir, err)
			for _, rec := range writes {
				errs[rec.OrchestrationID] = err
			}
			for _, id := range removes {
				errs[id] = err
			}
		}
	}
	if len(errs) > 0 {
		// Done again at the next flush: the write of a record kept, the
		// removal of one removed, as it stands then.
		mu.Lock()
		if st == s {
			for id := range errs {
				if _, ok := s.records[id]; ok {
					s.dirty[id] = true
				} else {
					s.removed[id] = true
				}
			}
		}
		mu.Unlock()
	}
	return errs
}

// handle applies what one message says. The caller holds mu.
func (s *state) handle(i any) {
	switch m := i.(type) {
	case *msgbus.NodeTombstoneUpdated:
		if m.Node != s.localhost {
			s.onPeerRecord(m.Value)
		}
		s.evaluateLocal(m.Path)
		s.removeIfDone(m.ID)
	case *msgbus.NodeTombstoneDeleted:
		s.removeIfDone(m.ID)
	case *msgbus.NodeMonitorUpdated:
		if m.Node == s.localhost {
			if !s.rejoined && s.localRejoined() {
				// Out of the rejoin, the away holder of the node
				// is looked at.
				s.rejoined = true
				s.evaluateLocal("")
			}
			return
		}
		away := m.Value.State.IsAwayFromObjects()
		if prev, ok := s.nodeAway[m.Node]; ok && prev == away {
			return
		}
		s.nodeAway[m.Node] = away
		if away {
			s.demoteHolder(m.Node)
		} else {
			// A node back may complete the cluster.
			s.removeAllIfDone()
		}
	case *msgbus.NodeMonitorDeleted:
		if m.Node != s.localhost {
			s.nodeAway[m.Node] = true
			s.demoteHolder(m.Node)
		}
	case *msgbus.ClusterConfigUpdated:
		s.evict()
		s.removeAllIfDone()
	case *msgbus.ConfigFileUpdated:
		s.evaluateLocal(m.Path.String())
	case *msgbus.InstanceConfigDeleted:
		if m.Node == s.localhost {
			s.evaluateLocal(m.Path.String())
		}
	}
}

// localRejoined says whether the node monitor of the node is out of its
// start and rejoin.
func (s *state) localRejoined() bool {
	mon := node.MonitorData.GetByNode(s.localhost)
	if mon == nil {
		return false
	}
	return !mon.State.IsOneOf(node.MonitorStateInit, node.MonitorStateRejoin)
}

// onPeerRecord merges the copy of a record a peer publishes into the local
// one. A record the node does not keep is adopted only with a pending
// holder: an all-cleared copy adopted would bring back a record the node
// removed. The caller holds mu.
func (s *state) onPeerRecord(rec tombstone.Record) {
	if _, ok := s.records[rec.OrchestrationID]; !ok {
		if !rec.HasPending() {
			return
		}
		if g, ok := s.gone[rec.OrchestrationID]; ok {
			// Removed here: a stale copy, in flight or published
			// again by a peer that missed the removal. Answered when
			// it names the node pending: its holder was cleared,
			// the node alone can say it.
			if g.by == "" || !rec.IsPending(s.localhost) {
				return
			}
			rec = rec.DeepCopy()
			h := rec.Holders[s.localhost]
			h.State = tombstone.HolderCleared
			h.ClearedBy = g.by
			rec.Holders[s.localhost] = h
			s.log.Infof("tombstone %s of %s: a peer publishes it again, holder %s cleared, %s", rec.OrchestrationID, rec.Path, s.localhost, g.by)
			s.upsert(rec)
			return
		}
		rec = rec.DeepCopy()
		if !s.localRejoined() {
			// Adopted before the rejoin, as by a node that was down:
			// its instance is not trusted to complete the delete.
			demote(&rec, s.localhost)
		}
	}
	s.upsert(rec)
}

// upsert merges rec into the local record of its id and, if it changed,
// publishes the result, written at the next flush. It says whether it
// changed. The caller holds mu.
func (s *state) upsert(rec tombstone.Record) bool {
	merged := rec.DeepCopy()
	if cur, ok := s.records[rec.OrchestrationID]; ok {
		var changed bool
		if merged, changed = tombstone.Merge(cur, rec); !changed {
			return false
		}
	}
	s.set(merged)
	s.dirty[merged.OrchestrationID] = true
	delete(s.removed, merged.OrchestrationID)
	s.publish(merged)
	return true
}

// set holds rec as the local record of its id, indexed by path. The caller
// holds mu.
func (s *state) set(rec tombstone.Record) {
	s.records[rec.OrchestrationID] = rec
	ids, ok := s.byPath[rec.Path]
	if !ok {
		ids = make(map[string]bool)
		s.byPath[rec.Path] = ids
	}
	ids[rec.OrchestrationID] = true
}

// unset forgets the local record of id. The caller holds mu.
func (s *state) unset(id string) {
	rec, ok := s.records[id]
	if !ok {
		return
	}
	delete(s.records, id)
	if ids, ok := s.byPath[rec.Path]; ok {
		delete(ids, id)
		if len(ids) == 0 {
			delete(s.byPath, rec.Path)
		}
	}
	delete(s.baseline, id)
}

// recordsOf returns the local records of path. The caller holds mu.
func (s *state) recordsOf(path string) []tombstone.Record {
	l := make([]tombstone.Record, 0, len(s.byPath[path]))
	for id := range s.byPath[path] {
		l = append(l, s.records[id])
	}
	return l
}

// publish holds the record as the one of the node and announces it. The
// caller holds mu.
func (s *state) publish(rec tombstone.Record) {
	tombstone.Data.Set(s.localhost, rec)
	s.enqueue(&msgbus.NodeTombstoneUpdated{Node: s.localhost, ID: rec.OrchestrationID, Path: rec.Path, Value: rec.DeepCopy()}, labels(s.localhost, rec.Path)...)
}

func labels(nodename, path string) []pubsub.Label {
	l := []pubsub.Label{{"node", nodename}, {"path", path}}
	if p, err := naming.ParsePath(path); err == nil {
		l = append(l, pubsub.Label{"namespace", p.Namespace})
	}
	return l
}

// demote turns the pending live holder nodename of rec away, and says
// whether it did.
func demote(rec *tombstone.Record, nodename string) bool {
	h, ok := rec.Holders[nodename]
	if !ok || h.State != tombstone.HolderPending || h.Mode == tombstone.HolderAway {
		return false
	}
	h.Mode = tombstone.HolderAway
	rec.Holders[nodename] = h
	return true
}

// demoteHolder turns away the pending live holder nodename of every local
// record: its node went away, so its instance is not trusted to complete
// the delete on its return. The caller holds mu.
func (s *state) demoteHolder(nodename string) {
	for _, rec := range s.records {
		rec = rec.DeepCopy()
		if !demote(&rec, nodename) {
			continue
		}
		s.log.Infof("tombstone %s of %s: holder %s away", rec.OrchestrationID, rec.Path, nodename)
		s.upsert(rec)
	}
}

// evict clears the holders no longer cluster nodes. The caller holds mu.
func (s *state) evict() {
	nodes := clusternode.Get()
	if len(nodes) == 0 {
		return
	}
	for _, rec := range s.records {
		rec = rec.DeepCopy()
		changed := false
		for n, h := range rec.Holders {
			if h.State == tombstone.HolderPending && !slices.Contains(nodes, n) {
				h.State = tombstone.HolderCleared
				h.ClearedBy = tombstone.ClearedByEvicted
				rec.Holders[n] = h
				changed = true
				s.log.Infof("tombstone %s of %s: holder %s evicted", rec.OrchestrationID, rec.Path, n)
			}
		}
		if changed {
			s.upsert(rec)
		}
	}
}

// clusterComplete says whether every cluster node is up: a node down may
// publish a pending copy of a record on its return, which a record removed
// meanwhile would not stop.
func clusterComplete() bool {
	nodes := clusternode.Get()
	if len(nodes) == 0 {
		return false
	}
	for _, n := range nodes {
		mon := node.MonitorData.GetByNode(n)
		if mon == nil || mon.State.IsAwayFromObjects() {
			return false
		}
	}
	return true
}

// removeIfDone removes the local record of id once all its holders are
// cleared, every cluster node is up, and no node publishes it with a pending
// holder. The caller holds mu.
//
// A daemon publishes its records as it starts, before its node monitor
// leaves the rejoin: an up node publishing no pending copy has none, so the
// record is removed without waiting for every node to publish the same
// all-cleared copy, which the first node removing its file would never let
// happen.
func (s *state) removeIfDone(id string) {
	rec, ok := s.records[id]
	if !ok || rec.HasPending() || !clusterComplete() {
		return
	}
	for _, n := range tombstone.Data.Publishers(id) {
		if c, ok := tombstone.Data.Get(n, id); ok && c.HasPending() {
			return
		}
	}
	s.unset(id)
	delete(s.dirty, id)
	s.removed[id] = true
	s.gone[id] = goneRecord{at: time.Now(), by: rec.Holders[s.localhost].ClearedBy}
	tombstone.Data.Unset(s.localhost, id)
	s.log.Infof("removed the tombstone %s of %s: all its holders cleared", id, rec.Path)
	s.enqueue(&msgbus.NodeTombstoneDeleted{Node: s.localhost, ID: id, Path: rec.Path}, labels(s.localhost, rec.Path)...)
}

// forgetGone forgets the records removed longer than removedMemory ago. The
// caller holds mu.
func (s *state) forgetGone() {
	for id, g := range s.gone {
		if time.Since(g.at) > removedMemory {
			delete(s.gone, id)
		}
	}
}

// removeAllIfDone tries the removal of every local record all cleared. The
// caller holds mu.
func (s *state) removeAllIfDone() {
	if !clusterComplete() {
		return
	}
	for id, rec := range s.records {
		if !rec.HasPending() {
			s.removeIfDone(id)
		}
	}
}

// Record writes rec, merged into the local record of its id, and returns
// once it is on stable storage: the caller deletes the instance only then.
func Record(rec tombstone.Record) error {
	mu.Lock()
	if st == nil {
		mu.Unlock()
		return ErrNotStarted
	}
	if rec.Holders == nil {
		rec.Holders = make(map[string]tombstone.Holder)
	}
	st.upsert(rec)
	s, n := st, st.queued
	mu.Unlock()
	if err := flush()[rec.OrchestrationID]; err != nil {
		return err
	}
	// The peers learn of it before the instance goes.
	s.waitPublished(n)
	return nil
}

// ClearLocal clears the holder of the node in the records of path pending
// it, as by.
func ClearLocal(path string, by tombstone.ClearedBy) error {
	mu.Lock()
	defer mu.Unlock()
	if st == nil {
		return ErrNotStarted
	}
	st.clear(path, []string{st.localhost}, by)
	return nil
}

// DemoteLocal turns away the pending live holder of the node in the record
// of the orchestration id of path: its instance failed to complete that
// delete or purge, so the operator decides, the issue shown. The other
// records of the path are left alone, and a record the node removed is not
// taken in again for it.
func DemoteLocal(path, id string) error {
	mu.Lock()
	defer mu.Unlock()
	if st == nil {
		return ErrNotStarted
	}
	if _, ok := st.gone[id]; ok {
		return nil
	}
	rec, ok := st.adoptPeerCopy(path, id)
	if !ok {
		return nil
	}
	if demote(&rec, st.localhost) {
		st.log.Infof("tombstone %s of %s: holder %s away, its instance failed to complete the %s", rec.OrchestrationID, rec.Path, st.localhost, rec.Kind)
		st.upsert(rec)
	}
	return nil
}

// adoptPeerCopy returns a copy of the local record of id of path, adopting
// the copy the peers publish if the node keeps none. The caller holds mu.
func (s *state) adoptPeerCopy(path, id string) (tombstone.Record, bool) {
	if rec, ok := s.records[id]; ok {
		return rec.DeepCopy(), true
	}
	rec, ok := tombstone.Data.MergedOf(path)[id]
	if !ok || !rec.HasPending() {
		return tombstone.Record{}, false
	}
	s.upsert(rec)
	return s.records[id].DeepCopy(), true
}

// adoptPeerCopies adopts the copies the peers publish of the records of
// path the node does not keep and that have a pending holder, even one it
// removed: an explicit clear of a stale copy is what ends it. The caller
// holds mu.
func (s *state) adoptPeerCopies(path string) {
	for id, rec := range tombstone.Data.MergedOf(path) {
		if _, ok := s.records[id]; ok || !rec.HasPending() {
			continue
		}
		s.upsert(rec)
	}
}

// reconcile adopts the copies the peers publish of the records the node
// does not keep, with a pending holder, as onPeerRecord does with their
// events: the data of the peers applied before the manager subscribed
// replays as events only as far as the start buffer goes. The caller holds
// mu.
func (s *state) reconcile() {
	for id, rec := range tombstone.Data.Merged() {
		if _, ok := s.records[id]; !ok && rec.HasPending() {
			s.onPeerRecord(rec)
		}
	}
}

// clear clears the pending holders nodes of the records of path as by,
// adopting first the copies the peers publish of the records the node does
// not keep, and returns the holders it cleared. The caller holds mu.
func (s *state) clear(path string, nodes []string, by tombstone.ClearedBy) []string {
	s.adoptPeerCopies(path)
	var cleared []string
	for _, rec := range s.recordsOf(path) {
		rec = rec.DeepCopy()
		changed := false
		for _, n := range nodes {
			h, ok := rec.Holders[n]
			if !ok || h.State != tombstone.HolderPending {
				continue
			}
			h.State = tombstone.HolderCleared
			h.ClearedBy = by
			rec.Holders[n] = h
			changed = true
			if !slices.Contains(cleared, n) {
				cleared = append(cleared, n)
			}
		}
		if !changed {
			continue
		}
		s.upsert(rec)
		s.log.Infof("tombstone %s of %s: holders %s cleared, %s", rec.OrchestrationID, rec.Path, nodes, by)
		s.removeIfDone(rec.OrchestrationID)
	}
	slices.Sort(cleared)
	return cleared
}

// Drop clears every pending holder of the records of path, as a create
// --force of the same name does, and returns the holders it cleared, once
// the records are written and published.
func Drop(path string) ([]string, error) {
	pending := tombstone.Data.PendingHolders(path, "")
	if len(pending) == 0 {
		// Nothing to drop, the manager running or not.
		return nil, nil
	}
	mu.Lock()
	if st == nil {
		mu.Unlock()
		return nil, ErrNotStarted
	}
	s := st
	ids := make([]string, 0)
	for id := range tombstone.Data.MergedOf(path) {
		ids = append(ids, id)
	}
	cleared := s.clear(path, pending, tombstone.ClearedByDropped)
	n := s.queued
	mu.Unlock()
	// Written and published before it returns, as Record: a crash once
	// the caller wrote the new config does not bring the record back.
	errs := flush()
	for _, id := range ids {
		if err := errs[id]; err != nil {
			return cleared, err
		}
	}
	s.waitPublished(n)
	return cleared, nil
}

// WasRemoved says whether the node removed the record of id lately: a copy
// of it a peer publishes is stale.
func WasRemoved(id string) bool {
	mu.Lock()
	defer mu.Unlock()
	if st == nil {
		return false
	}
	_, ok := st.gone[id]
	return ok
}

// List returns the records the nodes publish, the copies merged per id.
func List() []tombstone.Record {
	merged := tombstone.Data.Merged()
	l := make([]tombstone.Record, 0, len(merged))
	for _, rec := range merged {
		l = append(l, rec)
	}
	slices.SortFunc(l, func(a, b tombstone.Record) int {
		if a.Path != b.Path {
			if a.Path < b.Path {
				return -1
			}
			return 1
		}
		if a.OrchestrationID < b.OrchestrationID {
			return -1
		} else if a.OrchestrationID > b.OrchestrationID {
			return 1
		}
		return 0
	})
	return l
}
