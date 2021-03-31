package resfsflag

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/resource"
)

const (
	driverGroup = "fs"
	driverName  = "flag"
)

// T is the driver structure.
type T struct {
	resource.T
	Path     object.Path `json:"path"`
	lazyFile string      `json:"-"`
	lazyDir  string      `json:"-"`
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() resource.Manifest {
	return resource.Manifest{
		Group: driverGroup,
		Name:  driverName,
		Context: []resource.Context{
			{
				Key: "path",
				Ref: "object.path",
			},
		},
	}
}
