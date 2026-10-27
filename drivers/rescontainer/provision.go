package rescontainer

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
)

// ProvisionStarts says whether the provision of a container starts it: on the
// placement leader, and on every node for a standby resource.
//
// Off the leader, the provision of an instance stops what it started, the
// leader starting the instance. A container started there for that while
// needs what it mounts, which the resources the leader alone starts hold, as
// the shared vol a volume_mounts names: the path is not there, and the start
// fails the provision of the node. Starting it serves nothing either: the
// image is pulled at the first start, wherever the instance is placed then.
func ProvisionStarts(ctx context.Context, standby bool) bool {
	return standby || actioncontext.IsLeader(ctx)
}
