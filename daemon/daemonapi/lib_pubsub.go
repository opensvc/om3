package daemonapi

import (
	"time"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/plog"
)

func (a *DaemonAPI) announceSub(name string) {
	a.EventBus.Pub(&msgbus.ClientSubscribed{Time: time.Now(), Name: name}, a.LabelNode, labelAPI)
}

func (a *DaemonAPI) announceUnsub(name string) {
	a.EventBus.Pub(&msgbus.ClientUnsubscribed{Time: time.Now(), Name: name}, a.LabelNode, labelAPI)
}

func (a *DaemonAPI) announceNodeState(log *plog.Logger, state node.MonitorState) {
	log.Infof("announce node state %s", state)
	a.EventBus.Pub(&msgbus.SetNodeMonitor{Node: a.localhost, Value: node.MonitorUpdate{State: &state}}, labelAPI)
	time.Sleep(2 * daemondata.PropagationInterval())
}
