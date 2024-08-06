package resfsflag

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
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
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
		manifest.ContextTopology,
	)
	return m
}
