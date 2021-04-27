package object

import (
	"opensvc.com/opensvc/util/capabilities"
)

type (
	// NodeCapabilities contain the current node capabilities
	NodeCapabilities []string
)

// Render is a human rendered for node capabilities
func (t NodeCapabilities) Render() string {
	s := ""
	for _, c := range t {
		s = s + c + "\n"
	}
	return s
}

// NodeScanCapabilities scan node capabilities and return new capabilities
func (t Node) NodeScanCapabilities() (interface{}, error) {
	err := capabilities.Scan()
	if err != nil {
		return nil, err
	}
	return NodeCapabilities(capabilities.Data()), nil
}
