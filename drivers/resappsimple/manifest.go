package resappsimple

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/drivers/resapp"
)

var (
	drvID = driver.NewID(driver.GroupApp, "simple")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Add(
		manifest.ContextPath,
		manifest.ContextNodes,
		manifest.ContextObjectID,
	)
	m.AddKeywords(resapp.BaseKeywords...)
	m.AddKeywords(resapp.UnixKeywords...)
	m.AddKeywords(Keywords...)
	return m
}
