package daemonauth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

const (
	// vsockHostCID is the context id of the hypervisor, VMADDR_CID_HOST.
	vsockHostCID = 2

	// vsockReservedPorts bounds the source ports only a process with
	// CAP_NET_BIND_SERVICE can bind.
	vsockReservedPorts = 1024
)

type vsockStrategy struct{}

// Authenticate grants root to a connection of the hypervisor from a reserved
// source port. AF_VSOCK has no file mode, so any local user of the hypervisor
// can connect, and the reserved port tells its daemon from them. The context
// id and the port are the kernel's: a guest can not choose them, and the
// hypervisor is root on the guest whatever this says.
func (t vsockStrategy) Authenticate(ctx context.Context, _ *http.Request) (*Info, error) {
	cid, port, ok := daemonctx.VsockPeer(ctx)
	if !ok {
		return nil, fmt.Errorf("strategies/vsock: not a vsock client")
	}
	if cid != vsockHostCID {
		return nil, fmt.Errorf("strategies/vsock: context id %d is not the hypervisor", cid)
	}
	if port >= vsockReservedPorts {
		return nil, fmt.Errorf("strategies/vsock: source port %d is not reserved", port)
	}
	return &Info{
		Username: "hypervisor",
		Strategy: StrategyVsock,
		Grants:   []string{rbac.GrantRoot.String()},
	}, nil
}

func initVsock(_ context.Context, _ interface{}) (string, Strategy, error) {
	return "vsock auth", &vsockStrategy{}, nil
}
