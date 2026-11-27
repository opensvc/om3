package netwarden

import (
	"context"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/cgroupbpf"
	"github.com/opensvc/om3/v3/util/systemd"
)

// NodeCapability is the capability of a node systemd filters the network of
// its units on: it runs systemd, on a unified cgroup hierarchy, with a
// kernel attaching BPF programs to the cgroups.
const NodeCapability = "node.x.net_filter"

// hasCapability says the node has the capability, as of its last scan, and
// is replaced by the tests.
var hasCapability = func() bool {
	return capabilities.Has(NodeCapability)
}

func init() {
	capabilities.Register(capabilitiesScanner)
}

// capabilitiesScanner asks the kernel the programs of the root of the
// unified hierarchy: a kernel without BPF for cgroups, or a node whose
// hierarchy is not the unified one, refuses the query.
func capabilitiesScanner(ctx context.Context) ([]string, error) {
	if systemd.HasSystemd() && cgroupbpf.Supported() {
		return []string{NodeCapability}, nil
	}
	return nil, nil
}
