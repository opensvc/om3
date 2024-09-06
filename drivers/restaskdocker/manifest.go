package restaskdocker

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/rescontainer"
	"github.com/opensvc/om3/drivers/restask"
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
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
		manifest.ContextObjectID,
		manifest.ContextObjectID,
		manifest.ContextDNS,
		rescontainer.KWOsvcRootPath,
		rescontainer.KWGuestOS,
	)
	m.AddKeywords(restask.Keywords...)
	m.AddKeywords(Keywords...)
	return m
}
