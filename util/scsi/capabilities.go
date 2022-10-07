package scsi

import (
	"os/exec"

	"opensvc.com/opensvc/util/capabilities"
)

const (
	MpathPersistCapability = "node.x.scsi.mpathpersist"
	SGPersistCapability    = "node.x.scsi.sg_persist"
)

// CapabilitiesScanner is the capabilities scanner for scsi
func CapabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	if _, err := exec.LookPath("mpathpersist"); err != nil {
		// pass
	} else {
		l = append(l, MpathPersistCapability)
	}
	if _, err := exec.LookPath("sg_persist"); err != nil {
		// pass
	} else {
		l = append(l, SGPersistCapability)
	}
	return l, nil
}

// register node scanners
func init() {
	capabilities.Register(CapabilitiesScanner)
}
