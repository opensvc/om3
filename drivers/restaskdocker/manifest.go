package restaskdocker

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/rescontainer"
	"github.com/opensvc/om3/v3/drivers/restask"
	"github.com/opensvc/om3/v3/drivers/restaskocibase"
)

var (
	//go:embed text
	fs embed.FS
)

var (
	DrvID = driver.NewID(driver.GroupTask, "docker")
)

func init() {
	driver.Register(DrvID, New)
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(DrvID, t)
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
	m.AddKeywords(restaskocibase.Keywords...)
	m.Add(
		keywords.Keyword{
			Option:   "userns",
			Attr:     "UserNS",
			Scopable: true,
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/userns"),
		},
	)
	return m
}
