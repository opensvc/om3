package object

import (
	"opensvc.com/opensvc/util/capabilities"
	"path/filepath"
)

// OptsNodeScanCapabilities is the options of the NodeScanCapabilities function.
type OptsNodeScanCapabilities struct {
	Global OptsGlobal
}

// NodeScanCapabilities scan node capabilities and update node capability file
func (t Node) NodeScanCapabilities() (interface{}, error) {
	path := filepath.Join(t.VarDir(), "capabilities.json")
	return nil, capabilities.New(path).Scan()
}
