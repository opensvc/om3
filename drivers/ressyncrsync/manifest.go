package ressyncrsync

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/ressync"
)

var (
	drvID = driver.NewID(driver.GroupSync, "rsync")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextNodes,
		manifest.ContextDRPNodes,
		manifest.ContextTopology,
		manifest.ContextObjectID,
		manifest.ContextObjectPath,
	)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(Keywords...)
	return m
}
