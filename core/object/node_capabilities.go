package object

import (
	"github.com/opensvc/om3/util/capabilities"
)

// NodeScanCapabilities scan node capabilities and return new capabilities
func (t Node) NodeScanCapabilities() (capabilities.L, error) {
	err := capabilities.Scan()
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
