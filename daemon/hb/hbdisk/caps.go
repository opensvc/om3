package hbdisk

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/util/capabilities"
)

var (
	drvID = driver.NewID(driver.GroupHeartbeat, "disk")
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	return []string{drvID.Cap()}, nil
}
