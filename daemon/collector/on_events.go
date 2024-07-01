package collector

import (
	"time"

	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
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
		if len(t.instanceConfigChange) > 0 {
			if err := t.sendObjectConfigChange(); err != nil {
				t.log.Warnf("sendObjectConfigChange", err)
			}
		}
	} else {
		t.previousUpdatedAt = time.Time{}
		t.dropChanges()
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
		t.log.Errorf("can't setup requester: %s", err)
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
	delete(t.instanceConfigChange, c.Path)
}

func (t *T) onInstanceConfigUpdated(c *msgbus.InstanceConfigUpdated) {
	if !kindsConfigToPost.Has(c.Path.Kind) {
		return
	}
	t.instanceConfigChange[c.Path] = c
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
		if t.isSpeaker != c.Value.IsSpeaker {
			t.isSpeaker = c.Value.IsSpeaker
			t.publishOnChange(t.getState())
		}
	}
}

func (t *T) onObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	t.daemonStatusChange[c.Path.String()] = struct{}{}
}

func (t *T) onObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	t.daemonStatusChange[c.Path.String()] = struct{}{}
}
