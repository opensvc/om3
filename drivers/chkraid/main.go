// Package chkraid holds the raid check drivers, each reporting the state of
// the controllers, volumes and disks its tool manages, 0 when sound: MegaCli
// for LSI MegaRAID, sas2ircu for LSI SAS2, and the HP Smart Array cli.
package chkraid

import (
	"github.com/opensvc/om3/v3/core/check"
)

const (
	// DriverGroup is the type of the check drivers.
	DriverGroup = "raid"
)

func init() {
	check.Register(&megacli{})
	check.Register(&sas2ircu{})
	check.Register(&smartArray{})
}

func boolValue(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
