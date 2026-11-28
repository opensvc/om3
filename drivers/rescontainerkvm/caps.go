package rescontainerkvm

import (
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	l := make([]string, 0)
	drvCap := drvID.Cap()
	if _, err := exec.LookPath("machinectl"); err == nil {
		l = append(l, "node.x.machinectl")
	}
	if _, err := exec.LookPath("virsh"); err == nil {
		l = append(l, drvCap)
	}
	// The partitions capability says a guest placed in the process group
	// of its resource is held by its caps, which takes a libvirt with
	// partitions and a systemd that puts the machine where libvirt asks.
	if isPartitionsCapable(ctx) && partitionsRefusal(systemdVersion()) == "" {
		l = append(l, drvCap+".partitions")
	}
	if isHVMCapable(ctx) {
		l = append(l, drvCap+".hvm")
	}
	if isSerialLogCapable(ctx) {
		l = append(l, drvCap+serialLogCap)
	}
	return l, nil
}
