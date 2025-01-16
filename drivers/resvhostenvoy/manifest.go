package resvhostenvoy

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupVhost, "envoy")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		keywords.Keyword{
			Option:    "domains",
			Attr:      "Domains",
			Scopable:  true,
			Converter: converters.List,
			Default:   "{name}",
			Example:   "{name}",
			Text:      keywords.NewText(fs, "text/kw/domains"),
		},
		keywords.Keyword{
			Option:    "routes",
			Attr:      "Routes",
			Scopable:  true,
			Converter: converters.List,
			Example:   "route#1 route#2",
			Text:      keywords.NewText(fs, "text/kw/routes"),
		},
	)
	return m
}
