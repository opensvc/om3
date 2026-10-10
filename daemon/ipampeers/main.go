// Package ipampeers keeps, on this node, the addresses the other nodes
// report holding in the networks every node draws from.
//
// A draw reads the stores of the nodes alive and the status the cluster
// keeps of the others. The cluster drops the status of a node it stops
// hearing from, at once or after the maintenance grace period of a node
// stopped cleanly, and a node whose daemon restarts knows nothing of the
// others until they speak. What the node not heard from holds stays on its
// interfaces all the same, and a draw blind to it hands its addresses out
// twice.
//
// The records are what this node remembers of every peer, on disk: written
// from the statuses the peers publish, removed when a peer reports it no
// longer holds an address, deletes an object or leaves the cluster, and kept
// when the cluster merely stops hearing from it.
package ipampeers

import (
	"context"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	Manager struct {
		ctx       context.Context
		cancel    context.CancelFunc
		wg        sync.WaitGroup
		log       *plog.Logger
		sub       *pubsub.Subscription
		subQS     pubsub.QueueSizer
		localhost string

		allocators []*ipam.T
	}
)

// reconcileInterval is how often the records are checked against what the
// cluster reports, which catches what an event missed: an object a peer
// deleted while this node was not hearing it says so with no event.
const reconcileInterval = time.Minute

func New(subQS pubsub.QueueSizer) *Manager {
	return &Manager{subQS: subQS}
}

func (t *Manager) Start(parent context.Context) error {
	t.log = plog.NewDefaultLogger().WithPrefix("daemon: ipampeers: ").Attr("pkg", "daemon/ipampeers")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.localhost = hostname.Hostname()
	t.loadAllocators()

	sub := pubsub.SubFromContext(t.ctx, "daemon.ipampeers", t.subQS)
	sub.AddFilter(&msgbus.ClusterConfigUpdated{})
	sub.AddFilter(&msgbus.InstanceStatusUpdated{}, pubsub.Label{"from", "peer"})
	sub.AddFilter(&msgbus.InstanceStatusDeleted{}, pubsub.Label{"from", "peer"})
	sub.Start()
	t.sub = sub

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer func() {
			if err := t.sub.Stop(); err != nil && t.ctx.Err() == nil {
				t.log.Errorf("subscription stop: %s", err)
			}
		}()
		t.loop()
	}()
	return nil
}

func (t *Manager) Stop() error {
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *Manager) loop() {
	t.reconcile()
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.reconcile()
		case i := <-t.sub.C:
			switch c := i.(type) {
			case *msgbus.ClusterConfigUpdated:
				t.onClusterConfigUpdated(c)
			case *msgbus.InstanceStatusUpdated:
				t.setRecords(c.Node, c.Path, c.Value)
			case *msgbus.InstanceStatusDeleted:
				if !c.PeerDropAt.IsZero() {
					// The cluster stopped hearing from the node, which
					// is what the records outlive.
					continue
				}
				if inMaintenance(c.Node) {
					// A daemon stopping publishes the deletion of the
					// status of each of its instances, which go on running
					// all the same: the object is not deleted.
					continue
				}
				t.setRecords(c.Node, c.Path, instance.Status{})
			}
		}
	}
}

func (t *Manager) loadAllocators() {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		t.log.Warnf("load the networks: %s", err)
		return
	}
	t.allocators = network.ClusterWideAllocators(n)
}

func (t *Manager) onClusterConfigUpdated(c *msgbus.ClusterConfigUpdated) {
	if len(c.NetworkChanged) > 0 {
		t.loadAllocators()
	}
	if len(c.NetworkChanged) > 0 || len(c.NodesRemoved) > 0 {
		t.reconcile()
	}
}

// setRecords makes the records of an object on a peer what its status
// reports.
func (t *Manager) setRecords(nodename string, p naming.Path, st instance.Status) {
	if nodename == t.localhost {
		return
	}
	addrs := network.InstanceAddrs(t.allocators, p, st)
	for _, i := range t.allocators {
		written, removed, err := ipam.SetPeerRecords(ipam.PeerDir(i.Name), nodename, p.String(), addrs[i.Name])
		if err != nil {
			t.log.Warnf("network %s: record the addresses of %s on %s: %s", i.Name, p, nodename, err)
			continue
		}
		if written > 0 || removed > 0 {
			t.log.Debugf("network %s: %s on %s: %d address(es) recorded, %d removed", i.Name, p, nodename, written, removed)
		}
	}
}

// reconcile makes the records what the cluster reports of the peers it
// hears from, and drops the ones of the nodes that left the cluster. The
// records of a peer the cluster holds no data of are kept: that is the case
// they are for.
func (t *Manager) reconcile() {
	clusterNodes := make(map[string]bool)
	for _, nodename := range clusternode.Get() {
		clusterNodes[nodename] = true
	}
	// The nodes of the cluster are known once the cluster configuration is
	// read, which a reconcile at startup may come before: no node is taken
	// for one that left meanwhile.
	membersKnown := len(clusterNodes) > 0

	// This node has the data of every peer once it rejoined, which nmon
	// says when it has received it, or when the rejoin grace period ends.
	localStatus := node.StatusData.GetByNode(t.localhost)
	rejoined := localStatus != nil && !localStatus.RejoinedAt.IsZero()
	known := make(map[string]bool)
	reporting := make(map[string]bool)
	for _, e := range instance.StatusData.GetAll() {
		if e.Node == t.localhost {
			continue
		}
		known[e.Node+" "+e.Path.String()] = true
		reporting[e.Node] = true
		t.setRecords(e.Node, e.Path, *e.Value)
	}
	for _, i := range t.allocators {
		n, err := ipam.DropPeerRecords(ipam.PeerDir(i.Name), func(record ipam.PeerRecord) bool {
			if record.Node == t.localhost || (membersKnown && !clusterNodes[record.Node]) {
				return false
			}
			if node.StatusData.GetByNode(record.Node) == nil || inMaintenance(record.Node) {
				// Not heard from, or stopped: what it holds is not what
				// the cluster reports of it.
				return true
			}
			if !rejoined || !reporting[record.Node] {
				// The statuses of the peers are not all read yet, as just
				// after this daemon started: an object not reported yet
				// is no object gone.
				return true
			}
			p, ok := ipam.PathOfKey(record.Key)
			return ok && known[record.Node+" "+p.String()]
		})
		if err != nil {
			t.log.Warnf("network %s: reconcile the peer records: %s", i.Name, err)
		} else if n > 0 {
			t.log.Infof("network %s: %d peer record(s) of addresses no longer held dropped", i.Name, n)
		}
	}
}

// inMaintenance says the daemon of a node is stopping or stopped cleanly.
func inMaintenance(nodename string) bool {
	mon := node.MonitorData.GetByNode(nodename)
	return mon != nil && mon.State == node.MonitorStateMaintenance
}
