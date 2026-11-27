// Package netwarden keeps the network filters of the resources running on
// the node current.
//
// The start of a resource with rules, the ones of its net_rules keyword,
// resolves them and sets the filter on the slice of its process group. The rules naming an
// object or a namespace resolve to the addresses its resources publish,
// which change as the objects start, stop and move, and the ones naming a
// network or a node to what the cluster configuration says of them, and the
// ones reading a key of a datastore to its lines, which change with the
// configuration of the cfg or the sec holding it. The daemon hears of each
// change: it resolves the rules of the resources
// naming what changed again, and sets the filter of their slice when it
// differs, which systemd applies to the units running there. The filter of
// a guest is set on its network interfaces, by nftables, in one
// transaction: the elements of its sets replaced as the addresses move, and
// its table whole as the list of the rules changes.
//
// It works from the records the starts write, which hold the rules a
// filter was resolved from. The rules themselves follow the configuration:
// when the one of an object of the node changes, and as the daemon starts,
// the rules of its resources are read as a start would read them, and the
// ones that changed are resolved and set under the running resource, see
// converge. It is the one configuration change the daemon applies with no
// action asked: a filter is a security setting.
//
// And it works from events only, in their order: nothing is polled, no time
// of another node is compared, and the addresses of the objects are the
// ones their status events said, as the dns has them.
//
// What an event says is recorded as it is read, so the events are read at
// the pace they are published. What reads the node again and resolves every
// filter, a change of the nodes or the networks of the cluster, waits for
// the events queued behind it to be read, and is done once for all the
// changes read meanwhile, see Manager.handle: a daemon starting publishes
// them by tens, each saying the same of the filters. No clock decides when:
// the subscription having nothing more to read does.
//
// It says the filters it sets at the info level, and what it does not apply
// as a warning. What it weighs is said at the debug level, for an audit to
// show: a line by event, with the resources it has resolved again or why it
// concerns none, a line by resource resolved to the filter it has, and the
// records, the instances waited for and the nodes as they come and go. A
// line by decision, never one by address.
package netwarden

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	drvnetwarden "github.com/opensvc/om3/v3/drivers/shared/netwarden"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	// Manager is the daemon component keeping the filters current.
	Manager struct {
		ctx    context.Context
		cancel context.CancelFunc
		log    *plog.Logger
		sub    *pubsub.Subscription
		subQS  pubsub.QueueSizer
		wg     sync.WaitGroup

		// publisher tells the status of an instance is outdated, its
		// record written, nil when nobody is told.
		publisher pubsub.Publisher

		localhost string

		// entries is the index: the resources of the node a start
		// recorded a filter of, by the file of their record.
		entries map[string]*entry

		// base is the view of the networks and the nodes read from the
		// configuration of the node, nil when it is to be read again.
		base netrule.View

		// away is the nodes not telling their instances, lost or
		// their daemon away, as of the last event of each.
		away map[string]bool

		// heard is what the events said of the instances, see heard.
		heard heard

		// ghosts is the instances whose status went with no word of
		// what became of them, see ghosts.
		ghosts ghosts

		// tombstones is the records of the delete of the objects
		// heard of, see tombstones.
		tombstones tombstones

		// unread is the objects of the node whose configuration was
		// not read since the daemon started: their rules may have
		// changed while it was down. They are read between the
		// events, and an event of a configuration reads its own.
		unread []naming.Path

		// clusterChanged is the kind of the last event saying the
		// nodes or the networks of the cluster may have changed,
		// whose filters are not resolved again yet, see handle.
		clusterChanged string
	}

	// entry is a resource of the node with a filter recorded.
	entry struct {
		// file is the record of the resource.
		file string

		// path and rid name the resource.
		path naming.Path
		rid  string

		// rec is the record as last read or written.
		rec netrule.Record

		// objects and namespaces are the references of the rules: what
		// a change of has the rules resolved again.
		objects    map[naming.Path]bool
		namespaces map[string]bool

		// datastores are the cfg and sec objects the rules read
		// addresses from: a change of their configuration, which a
		// key set, changed or removed is, has the rules resolved
		// again.
		datastores map[naming.Path]bool

		// reapply has the filter set on the slice whether it changed or
		// not: the entry was indexed as the daemon started, which has
		// no memory of what the slice holds.
		reapply bool

		// rules says the rules of the record are ones the
		// configuration was found with, not written yet: the record
		// is written whether the filter changed or not.
		rules bool

		// unrecorded says the resource has no record yet, having
		// started with no rule: no stop removed the file.
		unrecorded bool
	}

	// backend is what the daemon asks of what applies a filter under a
	// running resource: the slice of a process group, or the network
	// interfaces of a guest.
	backend interface {
		// Set replaces the filter in force, an empty one lifting it.
		Set(filter netrule.Filter) error

		// HasFilter says a filter is in force.
		HasFilter() bool

		// HasMembers says something runs filtered, or would with a
		// filter: a unit of the slice, an interface of the guest.
		HasMembers() bool

		// HasProcs says something runs where no filter reaches it.
		HasProcs() bool
	}

	// sliceBackend is the backend of the slice of a process group.
	sliceBackend struct {
		slice
	}

	// netdevBackend is the backend of the interfaces of a guest.
	netdevBackend struct {
		drvnetwarden.Netdev
	}

	// slice is what the daemon asks of the slice of a process group.
	slice interface {
		SetIPFilter(allow, deny []string) error
		HasIPFilter() bool
		HasMembers() bool
		HasProcs() bool
	}
)

// recordName is the name of the record of a filter, in the directory of its
// resource.
const recordName = "netwarden.json"

var (
	// localView returns the view of the networks and the nodes of the
	// cluster, and is replaced by the tests.
	localView = netrule.LocalView

	// sliceOf returns the slice of a process group, and is replaced by
	// the tests, which set no property of the systemd of the host.
	sliceOf = func(id string) slice { return pg.Config{ID: id} }

	// netdevOf returns the backend of the network interfaces of a
	// guest, and is replaced by the tests, which load no table in the
	// nftables of the host.
	netdevOf = func(devs []string) backend { return netdevBackend{drvnetwarden.Netdev{Devs: devs}} }

	// recordFiles returns the records of the resources of an object of
	// the node, and is replaced by the tests.
	recordFiles = func(p naming.Path) []string {
		l, _ := filepath.Glob(filepath.Join(p.VarDir(), "*", recordName))
		return l
	}

	// installedPaths returns the objects of the node, and is replaced
	// by the tests.
	installedPaths = naming.InstalledPaths
)

// NewManager returns the component, not started.
func NewManager(subQS pubsub.QueueSizer) *Manager {
	return &Manager{
		subQS:      subQS,
		localhost:  hostname.Hostname(),
		entries:    make(map[string]*entry),
		away:       make(map[string]bool),
		heard:      make(heard),
		ghosts:     make(ghosts),
		tombstones: make(tombstones),
	}
}

// Start indexes the filters the resources of the node run with, and starts
// following the changes of what their rules name.
func (t *Manager) Start(parent context.Context) error {
	t.log = plog.NewDefaultLogger().WithPrefix("daemon: netwarden: ").Attr("pkg", "daemon/netwarden")
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)

	// A validation run by the daemon, as the one of a configuration
	// sent to its api, reads the objects from its data instead of asking
	// its own api.
	netrule.RegisterObjects(func(objects []naming.Path, namespaces []string) (netrule.Objects, netrule.Identities, error) {
		data := Objects(objects, namespaces)
		return data, Identities(data), nil
	})

	t.publisher = pubsub.PubFromContext(t.ctx)
	t.startSubscriptions()

	t.wg.Add(1)
	go func() {
		defer func() {
			if err := t.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Errorf("subscription stop: %s", err)
			}
			t.wg.Done()
		}()
		t.worker()
	}()
	t.log.Infof("started")
	return nil
}

// Stop ends the component. The filters stay as they are on the slices.
func (t *Manager) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.netwarden", t.subQS)
	sub.AddFilter(&msgbus.InstanceConfigUpdated{})
	sub.AddFilter(&msgbus.InstanceConfigDeleted{})
	sub.AddFilter(&msgbus.InstanceStatusUpdated{})
	sub.AddFilter(&msgbus.InstanceStatusDeleted{})
	sub.AddFilter(&msgbus.NodeNetworkStatusUpdated{})
	sub.AddFilter(&msgbus.NodeNetworkStatusDeleted{})
	sub.AddFilter(&msgbus.NodeDataUpdated{})
	sub.AddFilter(&msgbus.NodeTombstoneUpdated{})
	sub.AddFilter(&msgbus.NodeTombstoneDeleted{})
	sub.AddFilter(&msgbus.NodeMonitorUpdated{})
	sub.AddFilter(&msgbus.NodeMonitorDeleted{})
	sub.AddFilter(&msgbus.ClusterConfigUpdated{})
	sub.AddFilter(&msgbus.NodeConfigUpdated{})
	sub.AddFilter(&msgbus.JoinSuccess{})
	sub.AddFilter(&msgbus.ForgetPeer{})
	sub.AddFilter(&msgbus.AuditStart{})
	sub.AddFilter(&msgbus.AuditStop{})
	sub.Start()
	t.sub = sub
}

func (t *Manager) worker() {
	defer t.log.Tracef("done")
	t.onStart()
	for {
		if t.hasDeferred() {
			// What is deferred waits for the events: reading a
			// configuration as an action does, or resolving every
			// filter again, takes the time of many events, and
			// the subscription queues only so many.
			select {
			case <-t.ctx.Done():
				return
			case i := <-t.sub.C:
				t.handle(i)
			default:
				t.idle()
			}
			continue
		}
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			t.handle(i)
		}
	}
}

// handle follows an event read from the subscription, see onEvent, leaving
// for later what the events queued behind it would have done again.
//
// An event saying the nodes or the networks of the cluster may have changed
// has the configuration of the node read again, and every filter resolved
// again: it says nothing by itself, what it stands for being read from the
// node. A daemon starting, or a node joining, publishes tens of them in a
// row, the networks of each node one by one, and each was the whole work
// again. So the event only marks the view of the node as to be read again,
// which the filters resolved for another reason meanwhile do, and every
// filter is resolved again once, when no event waits, see idle.
//
// It changes no order that matters: what is resolved is what the node holds
// when it is, never what the event carried, so the change is seen as an
// event read a little later would have shown it. The events telling the
// instances and the nodes, whose order decides the addresses kept, are
// followed as they are read.
func (t *Manager) handle(i any) {
	switch c := i.(type) {
	case *msgbus.NodeNetworkStatusUpdated, *msgbus.NodeNetworkStatusDeleted, *msgbus.ClusterConfigUpdated, *msgbus.NodeConfigUpdated, *msgbus.JoinSuccess, *msgbus.ForgetPeer:
		t.base = nil
		t.clusterChanged = "a change of the cluster"
		if k, ok := c.(interface{ Kind() string }); ok {
			t.clusterChanged = k.Kind()
		}
	default:
		t.onEvent(i)
	}
}

// hasDeferred says something waits for no event to be waiting.
func (t *Manager) hasDeferred() bool {
	return t.clusterChanged != "" || len(t.unread) > 0
}

// idle does one of the things left for when no event waits, the filters to
// resolve again first, a change of the cluster being what a running filter
// is wrong for, and says there was one. It does one at a time, so the events
// published meanwhile are read between two.
func (t *Manager) idle() bool {
	if kind := t.clusterChanged; kind != "" {
		t.clusterChanged = ""
		t.resolveAll(kind)
		return true
	}
	return t.convergeNext()
}

// convergeNext applies the rules of the configuration of the next object of
// the node not read since the daemon started, and says there was one.
func (t *Manager) convergeNext() bool {
	if len(t.unread) == 0 {
		return false
	}
	p := t.unread[0]
	t.unread = t.unread[1:]
	t.converge(p)
	return true
}

// onStart indexes the records of the objects of the node, and sets their
// filters again: the slices may have lost them, and what the rules name may
// have changed while no daemon was there to hear of it. The rules
// themselves may have too: the configurations of the objects are left to be
// read between the events that follow, see convergeNext.
//
// The instance data of the daemon is still filling in then: what it does
// not tell yet is not known, and the addresses of the records are kept until
// it is, see netrule.Held.Keep and view.IsUnsure.
func (t *Manager) onStart() {
	// What the daemon already holds of the instances is not announced
	// again. The subscription is started: an event of an instance read
	// here says the same, or newer.
	for _, e := range instance.ConfigData.GetAll() {
		if e.Value != nil {
			t.heard.setConfig(e.Path, e.Node, e.Value.Scope, e.Value.ID, e.Value.UpdatedAt)
		}
	}
	for _, e := range instance.StatusData.GetAll() {
		if e.Value != nil {
			t.heard.setStatus(e.Path, e.Node, e.Value)
		}
	}
	for _, nodename := range clusterNodes() {
		t.away[nodename] = isNodeUnsure(nodename)
	}
	// The records of the deletes the daemon already holds are not
	// announced again either.
	for nodename, records := range tombstone.Data.All() {
		for id, rec := range records {
			if p, err := naming.ParsePath(rec.Path); err == nil {
				t.tombstoned(p, nodename, id, rec)
			}
		}
	}
	paths, err := installedPaths()
	if err != nil {
		t.log.Warnf("list the objects of the node: %s", err)
	}
	for _, p := range paths {
		t.scan(p)
	}
	for _, e := range t.entries {
		e.reapply = true
	}
	t.log.Debugf("starting: %d object(s) on the node, %d resource(s) with a filter recorded, set again if they run, %d node(s) away: %s", len(paths), len(t.entries), len(t.awayNodes()), strings.Join(t.awayNodes(), " "))
	t.refreshAll()
	t.unread = append(t.unread[:0], paths...)
}

// onEvent follows what changes the addresses the rules resolve to, and
// what says, or does not say, that an instance is gone.
//
// An instance status deleted is a word of the instance when its node is in
// the cluster: it deleted the instance. It is none when the node is lost,
// or its daemon away, stopping or starting: the instance becomes a ghost,
// whose addresses the filters keep, until its node tells it again or
// deletes it. A record of the delete of an object is remembered, and
// weighed against each address kept of its path: the ones of the object it
// deleted are not waited for, see tombstoned.
//
// The whole data a node sends is no word of the instances it lacks: a
// daemon back in the cluster sends it before it has found them all.
func (t *Manager) onEvent(i any) {
	switch c := i.(type) {
	case *msgbus.AuditStart:
		t.log.HandleAuditStart(c.Q, c.Subsystems, "netwarden")
	case *msgbus.AuditStop:
		t.log.HandleAuditStop(c.Q, c.Subsystems, "netwarden")
	case *msgbus.InstanceConfigUpdated:
		t.heard.setConfig(c.Path, c.Node, c.Value.Scope, c.Value.ID, c.Value.UpdatedAt)
		// The nodes of the object are known again: it is no ghost on
		// the others, where it was looked for while nothing said
		// where it runs.
		if n := t.ghosts.keepNodes(c.Path, c.Value.Scope); n > 0 {
			t.log.Debugf("%s@%s: its configuration names its nodes: %d instance(s) on other nodes are no longer waited for", c.Path, c.Node, n)
		}
		if c.Node == t.localhost {
			// The rules of its resources may be what changed. The
			// records are read once, before: the ones converge
			// writes are the ones it indexes.
			t.unread = slices.DeleteFunc(t.unread, func(p naming.Path) bool { return p == c.Path })
			t.scan(c.Path)
			t.converge(c.Path)
		}
		t.resolveAgain(c.Kind(), c.Path, c.Node, nil)
	case *msgbus.InstanceConfigDeleted:
		t.heard.dropConfig(c.Path, c.Node)
		if !t.isAway(c.Node) && t.ghosts.drop(c.Path, c.Node) {
			t.log.Debugf("%s@%s: deleted by its node: no longer waited for", c.Path, c.Node)
		}
		t.onInstanceChanged(c.Kind(), c.Path, c.Node, nil, true)
	case *msgbus.InstanceStatusUpdated:
		// A status evaluated again with its addresses unchanged, as
		// the ones an object with a short monitor schedule publishes
		// every second, changes no filter: only a record a start or a
		// stop of the node wrote is looked for.
		changed := t.heard.setStatus(c.Path, c.Node, &c.Value)
		if t.ghosts.drop(c.Path, c.Node) {
			t.log.Debugf("%s@%s: its node tells it again: no longer waited for", c.Path, c.Node)
			changed = true
		}
		t.onInstanceChanged(c.Kind(), c.Path, c.Node, &c.Value, changed)
	case *msgbus.InstanceStatusDeleted:
		t.heard.dropStatus(c.Path, c.Node)
		switch {
		case !c.PeerDropAt.IsZero():
			t.ghost(c.Path, c.Node, "its status went with its node, lost")
		case t.isAway(c.Node):
			t.ghost(c.Path, c.Node, "its status went while its node does not tell its instances")
		default:
			if t.ghosts.drop(c.Path, c.Node) {
				t.log.Debugf("%s@%s: deleted by its node: no longer waited for", c.Path, c.Node)
			}
		}
		t.onInstanceChanged(c.Kind(), c.Path, c.Node, nil, true)
	case *msgbus.NodeTombstoneUpdated:
		if p, err := naming.ParsePath(c.Path); err == nil {
			if t.tombstoned(p, c.Node, c.ID, c.Value) {
				t.onInstanceChanged(c.Kind(), p, c.Node, nil, true)
			}
		}
	case *msgbus.NodeTombstoneDeleted:
		// Removed by a node, every node up and none holding the
		// object deleted: nothing is left for the record to tell.
		if p, err := naming.ParsePath(c.Path); err == nil {
			t.tombstones.drop(p, c.ID)
		}
	case *msgbus.NodeDataUpdated:
		t.onNodeChanged(c.Kind(), c.Node, c.Value.Monitor.State.IsAwayFromObjects())
	case *msgbus.NodeMonitorUpdated:
		t.onNodeChanged(c.Kind(), c.Node, c.Value.State.IsAwayFromObjects())
	case *msgbus.NodeMonitorDeleted:
		t.onNodeChanged(c.Kind(), c.Node, true)
	case *msgbus.NodeNetworkStatusUpdated, *msgbus.NodeNetworkStatusDeleted, *msgbus.ClusterConfigUpdated, *msgbus.NodeConfigUpdated, *msgbus.JoinSuccess, *msgbus.ForgetPeer:
		kind := ""
		if k, ok := i.(interface{ Kind() string }); ok {
			kind = k.Kind()
		}
		t.onClusterChanged(kind)
	}
}

// tombstoned follows a record of the delete of an object, of the id given,
// published by the node given, and says it was not heard before.
//
// The record is the word the object it names is gone, from the nodes not
// heard of too: the instances of that object are no longer waited for, so
// the filters drop the addresses they kept of it at once, the node of an
// instance heard of or not, see view.IsUnsure. An allowed address goes on
// the evidence it is gone, which the record is, and waiting for the node
// would let more through, not less. A denied address stays until its object
// is deleted, which the record says, as the delete every node ran does: a
// node away may still run the instance until it is back, as a node deleting
// it last does.
//
// The object it names is the one of its id, as of the version deleted: the
// record is weighed against each address the filters remember of its path,
// by the object that published the address, as they are resolved again. An
// address of the object created again under the path, or restored since,
// is none of it, whatever the order the record and the instances are heard
// in: its instance is waited for as before.
//
// A record heard again, as each node holding it publishes it, and as a
// node starting does, is said once.
func (t *Manager) tombstoned(p naming.Path, nodename, id string, rec tombstone.Record) bool {
	if !t.tombstones.add(p, id, rec) {
		return false
	}
	of := "of no id told"
	if rec.ObjectID != "" {
		of = "of the id " + rec.ObjectID
	}
	t.log.Debugf("%s: a record of its delete is heard from %s, %s, as of %s: the instances of that object are no longer waited for", p, nodename, of, rec.Version.UTC().Format(time.RFC3339Nano))
	return true
}

// ghost remembers an instance whose status went with no word of what
// became of it, why saying how: the addresses it held are kept by the
// filters until something tells, as the record of the delete of the object
// that published them does, see view.IsUnsure.
func (t *Manager) ghost(p naming.Path, nodename, why string) {
	if t.ghosts.add(p, nodename) {
		t.log.Debugf("%s@%s: %s: waited for, the addresses it held kept", p, nodename, why)
	}
}

// awayNodes returns the nodes not telling their instances, sorted.
func (t *Manager) awayNodes() []string {
	l := make([]string, 0)
	for nodename, away := range t.away {
		if away {
			l = append(l, nodename)
		}
	}
	slices.Sort(l)
	return l
}

func (t sliceBackend) Set(filter netrule.Filter) error {
	if filter.HasPorts() {
		return errors.New("the filter of a slice knows no port")
	}
	// systemd knows no order: it is given the two lists letting
	// through what the steps do.
	allow, deny := filter.Flat()
	return t.SetIPFilter(netrule.Strings(allow), netrule.Strings(deny))
}

func (t sliceBackend) HasFilter() bool { return t.HasIPFilter() }

func (t netdevBackend) Set(filter netrule.Filter) error { return t.Update(filter) }
func (t netdevBackend) HasProcs() bool                  { return false }

// backendOf returns what applies the filter of a record.
func backendOf(rec netrule.Record) backend {
	if rec.Backend == netrule.BackendNetdev {
		return netdevOf(rec.Devs)
	}
	return sliceBackend{sliceOf(rec.PG)}
}

// inForce returns the filter of a record in the form its backend holds it:
// the steps, a set of each, on the interfaces of a guest, and the allowed
// and the denied list saying the same on a slice, systemd knowing no order.
//
// It is what tells a filter to set from one in force already: rules
// written in another order, or an address moving from a rule to another,
// are other steps, and often the same two lists.
func inForce(rec netrule.Record) netrule.Filter {
	if rec.Backend == netrule.BackendNetdev {
		return rec.Filter
	}
	return rec.Filter.Flattened()
}

// filtersNothing says the filter of a record is one its backend holds
// nothing of: no step of it names an address, or, on a slice, its steps let
// everything through, as a list ending with allow any whose deny rules
// resolve to nothing for now, which is no range to give systemd.
func filtersNothing(rec netrule.Record) bool {
	return inForce(rec).IsZero()
}

// where names what the filter of a record is set on: the slice of its
// process group, or the interfaces of its guest.
func where(rec netrule.Record) string {
	if rec.Backend == netrule.BackendNetdev {
		return strings.Join(rec.Devs, " ")
	}
	return filepath.Base(rec.PG)
}
