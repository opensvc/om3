package arrayfreenas

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	caps := []string{
		driver.NewID(driver.GroupArray, "truenas").Cap(),
		driver.NewID(driver.GroupArray, "freenas").Cap(), // backward compat
	}
	return caps, nil
}
