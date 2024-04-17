package restaskdocker

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
)

var (
	drvID    = driver.NewID(driver.GroupTask, "docker")
	altDrvID = driver.NewID(driver.GroupTask, "oci")
)

func init() {
	driver.Register(drvID, New)
	driver.Register(altDrvID, New)
}

// Manifest ...
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
		manifest.ContextObjectID,
		manifest.ContextObjectID,
		manifest.ContextDNS,
	)
	m.AddKeywords(Keywords...)
	return m
}
