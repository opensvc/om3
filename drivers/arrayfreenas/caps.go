package arrayfreenas

import (
	"context"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	caps := []string{
		driver.NewID(driver.GroupArray, "truenas").Cap(),
		driver.NewID(driver.GroupArray, "freenas").Cap(), // backward compat
	}
	return caps, nil
}
