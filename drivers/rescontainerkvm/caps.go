package rescontainerkvm

import (
	"os/exec"

	"opensvc.com/opensvc/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	drvCap := drvID.Cap()
	if _, err := exec.LookPath("machinectl"); err == nil {
		l = append(l, "node.x.machinectl")
	}
	if _, err := exec.LookPath("virsh"); err == nil {
		l = append(l, drvCap)
	}
	if isPartitionsCapable() {
		l = append(l, drvCap+".partitions")
	}
	if isHVMCapable() {
		l = append(l, drvCap+".hvm")
	}
	return l, nil
}
