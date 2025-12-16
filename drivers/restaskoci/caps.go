package rescontaineroci

import (
	"context"

	"github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	"github.com/opensvc/om3/v3/drivers/rescontainerpodman"
	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	l := make([]string, 0)
	if rescontainerdocker.IsGenuine(ctx) {
		l = append(l, drvID.Cap())
	} else if rescontainerpodman.IsGenuine() {
		l = append(l, drvID.Cap())
	}
	return l, nil
}
