package scsi

import (
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"

	"opensvc.com/opensvc/util/capabilities"
)

const (
	MpathPersistCapability = "node.x.scsi.mpathpersist"
	SGPersistCapability    = "node.x.scsi.sg_persist"
)

var (
	mpathReservationKeyFileRegexp = regexp.MustCompile(`(?m)^\s*reservation_key\s+("file"|file)\s*$`)
)

// CapabilitiesScanner is the capabilities scanner for scsi
func CapabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	if _, err := exec.LookPath("mpathpersist"); err != nil {
		// pass
	} else if mpathReservationKeyConfigured, err := isMpathReservationKeyConfigured(); err != nil {
		// pass
	} else if mpathReservationKeyConfigured {
		l = append(l, MpathPersistCapability)
	}
	if _, err := exec.LookPath("sg_persist"); err != nil {
		// pass
	} else {
		l = append(l, SGPersistCapability)
	}
	return l, nil
}

func isMpathReservationKeyConfigured() (bool, error) {
	b, err := ioutil.ReadFile("/etc/multipath.conf")
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return mpathReservationKeyFileRegexp.Match(b), nil
}

// register node scanners
func init() {
	capabilities.Register(CapabilitiesScanner)
}
