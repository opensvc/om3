package arraypure

import (
	"context"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/util/capabilities"
)

var (
	drvID = driver.NewID(driver.GroupArray, "pure")
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	return []string{drvID.Cap()}, nil
}
