package daemondatactx

import (
	"context"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
)

// DaemonData function returns new DaemonData from context DaemonDataCmd
func DaemonData(ctx context.Context) *daemondata.T {
	bus := daemonctx.DaemonDataCmd(ctx)
	return daemondata.New(bus)
}
