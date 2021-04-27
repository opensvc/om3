package object

import (
	"opensvc.com/opensvc/util/capabilities"
)

// NodeScanCapabilities scan node capabilities and return new capabilities
func (t Node) NodeScanCapabilities() (interface{}, error) {
	err := capabilities.Scan()
	if err != nil {
		return nil, err
	}
	return capabilities.Data(), nil
}
