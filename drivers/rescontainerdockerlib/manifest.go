package rescontainerdockerlib

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
)

var (
	drvID = driver.NewID(driver.GroupContainer, "dockerlib")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	return t.BT.ManifestWithID(drvID)
}
