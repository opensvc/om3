package resfsflag

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
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
	m.Add(
		manifest.ContextNodes,
		manifest.ContextTopology,
	)
	return m
}
