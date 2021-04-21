package resfsflag

import (
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/resource"
)

const (
	driverGroup = drivergroup.FS
	driverName  = "flag"
)

// T is the driver structure.
type T struct {
	resource.T
	Path     path.T   `json:"path"`
	Nodes    []string `json:"nodes"`
	lazyFile string
	lazyDir  string
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName)
	m.AddContext([]manifest.Context{
		{
			Key:  "path",
			Attr: "Path",
			Ref:  "object.path",
		},
		{
			Key:  "nodes",
			Attr: "Nodes",
			Ref:  "object.nodes",
		},
	}...)
	return m
}
