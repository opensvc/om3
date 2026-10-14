package resip

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
)

// ProvisionStarts says whether the provision of an address starts it: on the
// placement leader, and on every node for a standby resource.
//
// Off the leader, the provision of an instance stops what it started, the
// leader starting the instance. An address started there for that while is
// held by two nodes at once when it is not one of the node's own, as the one
// a lan network hands to the cluster, or one the configuration names: a
// duplicate on the segment, which an ipv6 node configuring it at the same
// time takes for an address in use, and never uses. Starting it serves
// nothing: an address drawn from a network is drawn whether it is up or not.
func ProvisionStarts(ctx context.Context, standby bool) bool {
	return standby || actioncontext.IsLeader(ctx)
}
