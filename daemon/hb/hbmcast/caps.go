package hbmcast

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/util/capabilities"
)

var (
	drvID = driver.NewID(driver.GroupHeartbeat, "multicast")
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	return []string{drvID.Cap()}, nil
}
