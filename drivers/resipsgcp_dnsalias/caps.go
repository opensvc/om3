package resipsgcp_dnsalias

import (
	"context"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/sgcp"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	if !file.Exists(sgcp.DefaultConfigPath) {
		return nil, nil
	}
	return []string{drvID.Cap()}, nil
}
