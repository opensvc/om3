package resfsflag

import (
	"opensvc.com/opensvc/core/drivergroup"
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
func (t T) Manifest() resource.Manifest {
	return resource.Manifest{
		Group: driverGroup,
		Name:  driverName,
		Context: []resource.Context{
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
		},
	}
}
