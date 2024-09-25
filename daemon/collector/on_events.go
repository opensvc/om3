package collector

import (
	"errors"
	"io/fs"
	"time"

	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/msgbus"
)

var (
	kindsConfigToPost = naming.NewKinds(naming.KindSvc, naming.KindVol)
)

func (t *T) onRefreshTicker() {
	if t.isSpeaker {
		err := t.sendCollectorData()
		if err != nil {
			t.log.Warnf("sendCollectorData: %s", err)
		}
		if len(t.objectConfigToSend) > 0 {
			if err := t.sendObjectConfigChange(); err != nil {
				t.log.Warnf("sendObjectConfigChange", err)
			}
		}
	} else {
		t.previousUpdatedAt = time.Time{}
		t.dropChanges()
	}
}

func (t *T) onClusterConfigUpdated(c *msgbus.ClusterConfigUpdated) {
	for _, nodename := range c.NodesAdded {
		t.clusterNode[nodename] = struct{}{}
	}
	for _, nodename := range c.NodesRemoved {
		delete(t.clusterNode, nodename)
	}
}

func (t *T) onConfigUpdated() {
	t.log.Debugf("reconfigure")
	if collector.Alive.Load() {
		t.log.Infof("disable collector clients")
		collector.Alive.Store(false)
	}
	err := t.setNodeFeedClient()
	if t.feedPinger != nil {
		t.feedPinger.Stop()
	}
	if err := t.setupRequester(); err != nil {
		if !errors.Is(err, object.ErrNodeCollectorConfig) {
			t.log.Errorf("can't setup requester: %s", err)
		}
	}
	if err != nil {
		t.log.Infof("the collector routine is dormant: %s", err)
	} else {
		t.log.Infof("feeding %s", t.feedClient)
		t.feedPinger = t.feedClient.NewPinger()
		time.Sleep(time.Microsecond * 10)
		t.feedPinger.Start(t.ctx, FeedPingerInterval)
	}
	t.publishOnChange(t.getState())
}

func (t *T) onInstanceConfigDeleted(c *msgbus.InstanceConfigDeleted) {
	if instanceConfig, ok := t.objectConfigToSend[c.Path]; ok {
		if instanceConfig == nil {
			// nothing to drop
			return
		}
		if instanceConfig.Node != c.Node {
			// don't drop yet, wait for event InstanceConfigDeleted from the same
			// node that emit the InstanceConfigUpdated.
			return
		}
		delete(t.objectConfigToSend, c.Path)
	}
}

func (t *T) onInstanceConfigUpdated(c *msgbus.InstanceConfigUpdated) {
	if !kindsConfigToPost.Has(c.Path.Kind) {
		return
	}
	sent, ok := t.objectConfigSent[c.Path]
	if !ok {
		sent.path = c.Path
		if err := sent.read(); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				t.log.Debugf("onInstanceConfigUpdated from %s@%s with checksum %s sent config cache absent", c.Path, c.Node, c.Value.Checksum)
			} else {
				t.log.Warnf("can't read sent config: %s", err)
			}
		} else {
			t.log.Debugf("onInstanceConfigUpdated from %s@%s with checksum %s init sent config cache %s", c.Path, c.Node, c.Value.Checksum, sent.Checksum)
			t.objectConfigSent[c.Path] = sent
		}
	}
	if sent.Checksum == c.Value.Checksum {
		// skip already sent config
		t.log.Debugf("onInstanceConfigUpdated from %s@%s skipped on same checksum %s", c.Path, c.Node, sent.Checksum)
		return
	}

	// Prefer the localhost objectConfigToSend if checksum match to avoid from
	// fetching config from peer when already present on localhost.
	// Example on node1 speaker:
	//     create object on node1
	//     => event 1: InstanceConfigUpdated{Node: <node1>
	//     => event 2: InstanceConfigUpdated{Node: <node2> (after node2 fetch config from node1) we can drop
	//                 event if its checksum value is same as event1 checksum.
	if toSend, ok := t.objectConfigToSend[c.Path]; ok && toSend != nil {
		// already have objectConfigToSend
		if toSend.Value.Checksum == c.Value.Checksum {
			if c.Node != t.localhost {
				t.log.Debugf("onInstanceConfigUpdated from %s@%s skipped: found localhost InstanceConfigUpdated with same checksum %s", c.Path, c.Node, sent.Checksum)
				return
			}
		}
	}

	t.log.Debugf("onInstanceConfigUpdated from %s@%s checksum %s need send", c.Path, c.Node, c.Value.Checksum)
	t.objectConfigToSend[c.Path] = c
}

func (t *T) onInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	i := instance.InstanceString(c.Path, c.Node)
	delete(t.changes.instanceStatusUpdates, i)
	delete(t.instances, i)
	t.changes.instanceStatusDeletes[i] = c
	t.daemonStatusChange[i] = struct{}{}
}

func (t *T) onInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) {
	i := instance.InstanceString(c.Path, c.Node)
	delete(t.changes.instanceStatusDeletes, i)
	t.changes.instanceStatusUpdates[i] = c
	t.instances[i] = struct{}{}
	t.daemonStatusChange[i] = struct{}{}
}

func (t *T) onNodeConfigUpdated(c *msgbus.NodeConfigUpdated) {
	t.onConfigUpdated()
}

func (t *T) onNodeMonitorDeleted(c *msgbus.NodeMonitorDeleted) {
	delete(t.nodeFrozenAt, c.Node)
	t.daemonStatusChange[c.Node] = struct{}{}
}

func (t *T) onNodeStatusUpdated(c *msgbus.NodeStatusUpdated) {
	if c.Value.FrozenAt != t.nodeFrozenAt[c.Node] {
		t.nodeFrozenAt[c.Node] = c.Value.FrozenAt
		t.daemonStatusChange[c.Node] = struct{}{}
	}
	if c.Node == t.localhost {
		isSpeaker := !t.disable && c.Value.IsLeader
		if isSpeaker != t.isSpeaker {
			t.isSpeaker = isSpeaker
			t.publishOnChange(t.getState())
		}
	}
}

func (t *T) onObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	t.daemonStatusChange[c.Path.String()] = struct{}{}
	delete(t.clusterObject, c.Path.String())
}

func (t *T) onObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	t.daemonStatusChange[c.Path.String()] = struct{}{}
	t.clusterObject[c.Path.String()] = struct{}{}
}
