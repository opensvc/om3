// Package tombmon keeps the tombstone records of the node: it loads them at
// start and publishes them, writes and publishes the record of a delete
// before the instance goes, and removes a record once it can concern no
// node any more: as soon as each node it lists was seen deleting its
// instance in the orchestration, or a day later.
//
// A record never changes and is never copied from a peer: each node writes
// and removes its own.
package tombmon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/file"
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
		wg     sync.WaitGroup
		subQS  pubsub.QueueSizer
	}
)

var (
	// Retention is how long a record is kept at least, from its write on
	// the node: a peer may hold a config of the version deleted that it
	// has not published yet, as one whose config manager had not started
	// as the delete ran, which no other signal tells. A record kept longer
	// does no harm: it concerns the version deleted only.
	Retention = 24 * time.Hour

	// sweepInterval is the pace of the removal of the records.
	sweepInterval = time.Minute

	// ackRetention is how long the delete of an instance by a peer is
	// remembered, for a record written just after it.
	ackRetention = 10 * time.Minute

	// acks are the nodes seen deleting their instance, by orchestration
	// id, and when: in memory only, a record loaded at start waiting
	// Retention.
	acks = make(map[string]map[string]time.Time)

	mu        sync.Mutex
	dir       string
	localhost string
	publisher pubsub.Publisher
	log       *plog.Logger
	started   bool

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
// monitor leaves the rejoin. They are not trusted: the node was down, and
// a record of its own may be one of a delete it did not complete.
func (t *Manager) Start(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)
	mu.Lock()
	dir = tombstone.Dir()
	localhost = hostname.Hostname()
	publisher = pubsub.PubFromContext(t.ctx)
	log = t.log
	started = true
	mu.Unlock()
	records, err := tombstone.Load(dir)
	if err != nil {
		t.log.Errorf("load %s: %s", dir, err)
	}
	for _, rec := range records {
		publish(rec, false)
		t.log.Infof("loaded the tombstone %s of the %s of %s", rec.OrchestrationID, rec.Kind, rec.Path)
	}
	sub := pubsub.SubFromContext(t.ctx, "daemon.tombmon", t.subQS)
	sub.AddFilter(&msgbus.InstanceMonitorUpdated{})
	sub.Start()
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer func() {
			if err := sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Warnf("subscription stop: %s", err)
			}
		}()
		ticker := time.NewTicker(sweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-t.ctx.Done():
				return
			case <-ticker.C:
				sweep()
			case i := <-sub.C:
				if ev, ok := i.(*msgbus.InstanceMonitorUpdated); ok {
					onInstanceMonitorUpdated(ev)
				}
			}
		}
	}()
	return nil
}

func (t *Manager) Stop() error {
	t.cancel()
	t.wg.Wait()
	mu.Lock()
	started = false
	mu.Unlock()
	return nil
}

// Record writes rec, synced, and publishes it, trusted: the caller deletes
// the instance only once it returned with no error.
func Record(rec tombstone.Record) error {
	mu.Lock()
	defer mu.Unlock()
	if !started {
		return ErrNotStarted
	}
	if err := tombstone.Write(dir, rec); err != nil {
		return err
	}
	if err := tombstone.SyncDir(dir); err != nil {
		return err
	}
	publish(rec, true)
	return nil
}

func publish(rec tombstone.Record, trusted bool) {
	tombstone.Data.Set(localhost, rec, trusted)
	publisher.Pub(&msgbus.NodeTombstoneUpdated{Node: localhost, ID: rec.OrchestrationID, Path: rec.Path, Value: rec}, labels(rec.Path)...)
}

func labels(path string) []pubsub.Label {
	l := []pubsub.Label{{"node", localhost}, {"path", path}}
	if p, err := naming.ParsePath(path); err == nil {
		l = append(l, pubsub.Label{"namespace", p.Namespace})
	}
	return l
}

// onInstanceMonitorUpdated remembers a peer deleting its instance in an
// orchestration, the delete done: the instance monitor of the peer says so,
// a positive sign no peer dropped or restarting gives. A record of the
// orchestration it completes goes at once.
func onInstanceMonitorUpdated(ev *msgbus.InstanceMonitorUpdated) {
	if ev.Value.State != instance.MonitorStateDeleteSuccess || ev.Value.OrchestrationID == uuid.Nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if !started || ev.Node == localhost {
		return
	}
	id := ev.Value.OrchestrationID.String()
	now := time.Now()
	for k, m := range acks {
		for n, at := range m {
			if now.Sub(at) > ackRetention {
				delete(m, n)
			}
		}
		if len(m) == 0 {
			delete(acks, k)
		}
	}
	if acks[id] == nil {
		acks[id] = make(map[string]time.Time)
	}
	acks[id][ev.Node] = now
	if rec, ok := tombstone.Data.Get(localhost, id); ok && clusterUp() && removable(id, rec) {
		remove(id, rec)
		syncDir()
	}
}

// sweep removes the records of the node that concern no node any more:
// every cluster node up, no node publishing a config they kill, and each
// node they list seen deleting its instance, or kept Retention.
func sweep() {
	mu.Lock()
	defer mu.Unlock()
	if !started || !clusterUp() {
		return
	}
	n := 0
	for id, rec := range tombstone.Data.GetByNode(localhost) {
		if !removable(id, rec) {
			continue
		}
		remove(id, rec)
		n++
	}
	if n > 0 {
		syncDir()
	}
}

// remove removes the record of id of the node. The caller holds mu, and
// syncs the directory.
func remove(id string, rec tombstone.Record) {
	if err := tombstone.RemoveFile(dir, id); err != nil {
		log.Errorf("remove the tombstone %s: %s", id, err)
		return
	}
	tombstone.Data.Unset(localhost, id)
	delete(acks, id)
	publisher.Pub(&msgbus.NodeTombstoneDeleted{Node: localhost, ID: id, Path: rec.Path}, labels(rec.Path)...)
	log.Infof("removed the tombstone %s of the %s of %s", id, rec.Kind, rec.Path)
}

func syncDir() {
	if err := tombstone.SyncDir(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Errorf("sync %s: %s", dir, err)
	}
}

// acked says whether each node the record lists was seen deleting its
// instance in the orchestration of the record.
func acked(rec tombstone.Record) bool {
	if len(rec.Nodes) == 0 {
		return false
	}
	m := acks[rec.OrchestrationID]
	for _, n := range rec.Nodes {
		if _, ok := m[n]; !ok {
			return false
		}
	}
	return true
}

// removable says whether the record of id may go: no node publishing a
// config it kills, and each node it lists seen deleting its instance, or
// kept Retention on the local clock, from the modification time of its
// file. The caller holds mu.
func removable(id string, rec tombstone.Record) bool {
	p, err := naming.ParsePath(rec.Path)
	if err != nil {
		return true
	}
	if !acked(rec) {
		written := file.ModTime(filepath.Join(dir, id+".json"))
		if written.IsZero() || time.Since(written) < Retention {
			return false
		}
	}
	for _, cfg := range instance.ConfigData.GetByPath(p) {
		if rec.Kills(rec.Path, cfg.UpdatedAt) {
			return false
		}
	}
	return true
}

// clusterUp says whether every cluster node is up: a node down may hold a
// config the record kills, which it publishes on its return.
func clusterUp() bool {
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

// List returns the records of every node, sorted by path, id and node.
func List() []Entry {
	var l []Entry
	for node, recs := range tombstone.Data.All() {
		for _, rec := range recs {
			l = append(l, Entry{Node: node, Record: rec})
		}
	}
	slices.SortFunc(l, func(a, b Entry) int {
		if c := strings.Compare(a.Record.Path, b.Record.Path); c != 0 {
			return c
		}
		if c := strings.Compare(a.Record.OrchestrationID, b.Record.OrchestrationID); c != 0 {
			return c
		}
		return strings.Compare(a.Node, b.Node)
	})
	return l
}

// Entry is a record and the node publishing it.
type Entry struct {
	Node   string
	Record tombstone.Record
}
