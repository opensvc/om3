package client

import "opensvc.com/opensvc/core/client/api"

func (t T) NewGetDaemonStats() *api.GetDaemonStats {
	return api.NewGetDaemonStats(t)
}

func (t T) NewGetDaemonStatus() *api.GetDaemonStatus {
	return api.NewGetDaemonStatus(t)
}

func (t T) NewGetDaemonRunning() *api.GetDaemonRunning {
	return api.NewGetDaemonRunning(t)
}

func (t T) NewPostDaemonStop() *api.PostDaemonStop {
	return api.NewPostDaemonStop(t)
}

func (t T) NewGetEvents() *api.GetEvents {
	return api.NewGetEvents(t)
}

func (t T) NewGetSchedules() *api.GetSchedules {
	return api.NewGetSchedules(t)
}

func (t T) NewGetKey() *api.GetKey {
	return api.NewGetKey(t)
}

func (t T) NewGetNodesInfo() *api.GetNodesInfo {
	return api.NewGetNodesInfo(t)
}

func (t T) NewGetNodeLog() *api.GetNodeLog {
	return api.NewGetNodeLog(t)
}

func (t T) NewGetNodeBacklog() *api.GetNodeBacklog {
	return api.NewGetNodeBacklog(t)
}

func (t T) NewGetObjectsLog() *api.GetObjectsLog {
	return api.NewGetObjectsLog(t)
}

func (t T) NewGetObjectsBacklog() *api.GetObjectsBacklog {
	return api.NewGetObjectsBacklog(t)
}

func (t T) NewGetObjectConfig() *api.GetObjectConfig {
	return api.NewGetObjectConfig(t)
}

func (t T) NewGetObjectConfigFile() *api.GetObjectConfigFile {
	return api.NewGetObjectConfigFile(t)
}

func (t T) NewGetObjectSelector() *api.GetObjectSelector {
	return api.NewGetObjectSelector(t)
}

func (t T) NewGetObjectStatus() *api.GetObjectStatus {
	return api.NewGetObjectStatus(t)
}

func (t T) NewGetPools() *api.GetPools {
	return api.NewGetPools(t)
}

func (t T) NewGetNetworks() *api.GetNetworks {
	return api.NewGetNetworks(t)
}

func (t T) NewGetRelayMessage() *api.GetRelayMessage {
	return api.NewGetRelayMessage(t)
}

func (t T) NewPostKey() *api.PostKey {
	return api.NewPostKey(t)
}

func (t T) NewPostNodeAction() *api.PostNodeAction {
	return api.NewPostNodeAction(t)
}

func (t T) NewPostNodeClear() *api.PostNodeClear {
	return api.NewPostNodeClear(t)
}

func (t T) NewPostNodeMonitor() *api.PostNodeMonitor {
	return api.NewPostNodeMonitor(t)
}

func (t T) NewPostObjectAbort() *api.PostObjectAbort {
	return api.NewPostObjectAbort(t)
}

func (t T) NewPostObjectAction() *api.PostObjectAction {
	return api.NewPostObjectAction(t)
}

func (t T) NewPostObjectClear() *api.PostObjectClear {
	return api.NewPostObjectClear(t)
}

func (t T) NewPostObjectCreate() *api.PostObjectCreate {
	return api.NewPostObjectCreate(t)
}

func (t T) NewPostObjectMonitor() *api.PostObjectMonitor {
	return api.NewPostObjectMonitor(t)
}

func (t T) NewPostObjectStatus() *api.PostObjectStatus {
	return api.NewPostObjectStatus(t)
}

func (t T) NewPostObjectSwitchTo() *api.PostObjectSwitchTo {
	return api.NewPostObjectSwitchTo(t)
}

func (t T) NewPostRelayMessage() *api.PostRelayMessage {
	return api.NewPostRelayMessage(t)
}

func (t T) NewPostRunDone() *api.PostRunDone {
	return api.NewPostRunDone(t)
}
