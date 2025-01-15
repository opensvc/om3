package resappforking

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/resapp"
)

var (
	drvID = driver.NewID(driver.GroupApp, "forking")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, &t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
		manifest.ContextObjectID,
	)
	m.AddKeywords(resapp.BaseKeywords...)
	m.AddKeywords(resapp.UnixKeywords...)
	m.AddKeywords(Keywords...)
	return m
}
