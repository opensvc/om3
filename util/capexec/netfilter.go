package capexec

import (
	"fmt"
	"path/filepath"

	"github.com/opensvc/om3/v3/util/cgroupbpf"
)

// netFilterQuerier asks the kernel, and is replaced by the tests.
var netFilterQuerier cgroupbpf.Querier = cgroupbpf.Kernel{}

// netFilterDir returns the cgroup directory om exec runs in, and is replaced
// by the tests.
var netFilterDir = func() (string, error) {
	return cgroupbpf.Dir(0)
}

// checkNetFilter refuses a command asked to run filtered when no network
// filter is attached to the cgroup it runs in.
//
// om exec is the main process of the unit of the resource then, in the
// cgroup systemd made for the unit, with the programs enforcing the filter
// attached before any process of the unit ran. The cgroup above is the one
// of the slice, which systemd attaches nothing to: a filter counted there
// too is not the one of the unit.
func (t T) checkNetFilter() error {
	if t.NetFilter == nil || !*t.NetFilter {
		return nil
	}
	dir, err := netFilterDir()
	if err != nil {
		return fmt.Errorf("--net-filter: %w", err)
	}
	if err := cgroupbpf.Verify(netFilterQuerier, dir, filepath.Dir(dir)); err != nil {
		return fmt.Errorf("--net-filter: %w: the command is not run unfiltered", err)
	}
	return nil
}
