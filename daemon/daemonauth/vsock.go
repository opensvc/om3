package daemonauth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/core/vsockauth"
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

// vsockStrategy says where the guest reads what it verifies a hypervisor with.
type vsockStrategy struct {
	dmiEntries  string
	productUUID string
}

// Authenticate grants root to a connection of the hypervisor from a reserved
// source port, holding a token of the key the firmware of the guest names.
//
// AF_VSOCK has no file mode, so any local user of the hypervisor can connect,
// and the reserved port, the context id and the port being the kernel's, only
// tells a process with CAP_NET_BIND_SERVICE from them. The token tells the
// daemon of the hypervisor from that process, who is root on the guest anyway:
// see core/vsockauth. A guest whose firmware holds no key grants nothing, and
// is reached the other ways.
func (t vsockStrategy) Authenticate(ctx context.Context, r *http.Request) (*Info, error) {
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
	token, err := bearerToken(r)
	if err != nil {
		return nil, fmt.Errorf("strategies/vsock: %w", err)
	}
	key, err := vsockauth.GuestKey(t.dmiEntries)
	if err != nil {
		return nil, fmt.Errorf("strategies/vsock: %w", err)
	}
	guest, err := vsockauth.GuestUUID(t.productUUID)
	if err != nil {
		return nil, fmt.Errorf("strategies/vsock: %w", err)
	}
	if err := vsockauth.Verify(token, key, guest); err != nil {
		return nil, fmt.Errorf("strategies/vsock: %w", err)
	}
	return &Info{
		Username: "hypervisor",
		Strategy: StrategyVsock,
		Grants:   []string{rbac.GrantRoot.String()},
	}, nil
}

func initVsock(_ context.Context, _ interface{}) (string, Strategy, error) {
	return "vsock auth", &vsockStrategy{
		dmiEntries:  vsockauth.DMIEntries,
		productUUID: vsockauth.ProductUUID,
	}, nil
}
