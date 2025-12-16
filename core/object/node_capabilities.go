package object

import (
	"context"

	"github.com/opensvc/om3/v3/util/capabilities"
)

// ScanCapabilities scan node capabilities and return new capabilities
func (t Node) ScanCapabilities(ctx context.Context) (capabilities.L, error) {
	err := capabilities.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return capabilities.Data(), nil
}

// PrintCapabilities load and return node capabilities
func (t Node) PrintCapabilities() (capabilities.L, error) {
	caps, err := capabilities.Load()
	if err != nil {
		return nil, err
	}
	return caps, nil
}
