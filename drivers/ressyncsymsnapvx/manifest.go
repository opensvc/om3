package ressyncsymsnapvx

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/ressync"
)

var (
	drvID = driver.NewID(driver.GroupSync, "symsnapvx")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.Add(manifest.ContextObjectFQDN)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(Keywords...)
	return m
}
