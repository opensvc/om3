package daemondatactx

import (
	"context"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
)

// DaemonData function returns new DaemonData from context DaemonDataCmd
func DaemonData(ctx context.Context) *daemondata.T {
	return daemondata.New(daemonctx.DaemonDataCmd(ctx))
}
