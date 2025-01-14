package resiproute

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupIP, "route")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		manifest.ContextObjectPath,
		keywords.Keyword{
			Attr:     "NetNS",
			Example:  "container#0",
			Option:   "netns",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/netns"),
		},
		keywords.Keyword{
			Attr:     "Gateway",
			Option:   "gateway",
			Example:  "1.2.3.4",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/gateway"),
		},
		keywords.Keyword{
			Attr:     "To",
			Example:  "192.168.100.0/24",
			Option:   "to",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/to"),
		},
		keywords.Keyword{
			Attr:        "Dev",
			DefaultText: keywords.NewText(fs, "text/kw/dev.default"),
			Example:     "eth1",
			Option:      "dev",
			Required:    false,
			Scopable:    true,
			Text:        keywords.NewText(fs, "text/kw/dev"),
		},
	)
	return m
}
