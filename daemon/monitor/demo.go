package monitor

import (
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/timestamp"
)

var (
	// For demo
	demoAvails = map[string]string{
		"dev1n1":        "undef",
		"dev1n2":        "undef",
		"dev1n3":        "undef",
		"u2004-local-1": "undef",
		"u2004-local-2": "undef",
		"u2004-local-3": "undef",
	}
	demoSvc = "demo"
	mode    = "undef"
)

func (t *T) demoLoop() {
	// For demo
	dataCmd := daemondata.FromContext(t.ctx)
	dataCmd.PushOps([]jsondelta.Operation{
		{
			OpPath:  jsondelta.OperationPath{"monitor", "status_updated"},
			OpValue: jsondelta.NewOptValue(timestamp.Now()),
			OpKind:  "replace",
		},
	})
	status := dataCmd.GetStatus()
	for remote, v := range demoAvails {
		remoteNodeStatus := status.GetNodeStatus(remote)
		if remoteNodeStatus != nil {
			if demoStatus, ok := remoteNodeStatus.Services.Status[demoSvc]; ok {
				if v != demoStatus.Avail.String() {
					t.log.Info().Msgf("%s@%s status changed from %s -> %s", demoSvc, remote, v, demoStatus.Avail.String())
					demoAvails[remote] = demoStatus.Avail.String()
				}
			}
		}
	}
}
