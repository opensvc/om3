package resfsflag

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/manifest"
)

var (
	drvID = driver.NewID(driver.GroupFS, "flag")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
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
		{
			Key:  "topology",
			Attr: "Topology",
			Ref:  "object.topology",
		},
	}...)
	return m
}
