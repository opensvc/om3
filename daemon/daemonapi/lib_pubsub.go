package daemonapi

import (
	"time"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/plog"
)

func (a *DaemonAPI) announceSub(name string) {
	a.Pub.Pub(&msgbus.ClientSubscribed{Time: time.Now(), Name: name}, a.LabelLocalhost, labelOriginAPI)
}

func (a *DaemonAPI) announceUnsub(name string) {
	a.Pub.Pub(&msgbus.ClientUnsubscribed{Time: time.Now(), Name: name}, a.LabelLocalhost, labelOriginAPI)
}

func (a *DaemonAPI) announceNodeState(log *plog.Logger, state node.MonitorState) {
	log.Infof("announce node state %s", state)
	a.Pub.Pub(&msgbus.SetNodeMonitor{Node: a.localhost, Value: node.MonitorUpdate{State: &state}}, labelOriginAPI)
	time.Sleep(2 * daemondata.PropagationInterval())
}
