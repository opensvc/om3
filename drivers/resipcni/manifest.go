//go:build linux

package resipcni

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/resip"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupIP, "cni")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		manifest.ContextCNIPlugins,
		manifest.ContextCNIConfig,
		manifest.ContextObjectID,
		manifest.ContextObjectPath,
		manifest.ContextObjectFQDN,
		manifest.ContextDNS,
		resip.KeywordWaitDNS,
		keywords.Keyword{
			Attr:      "Expose",
			Converter: converters.List,
			Example:   "443/tcp:8443 53/udp",
			Option:    "expose",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/expose"),
		},
		keywords.Keyword{
			Attr:     "Network",
			Default:  "default",
			Example:  "mynet",
			Option:   "network",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/network"),
		},
		keywords.Keyword{
			Aliases:  []string{"ipdev"},
			Attr:     "NSDev",
			Default:  "eth12",
			Example:  "front",
			Option:   "nsdev",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/nsdev"),
		},
		keywords.Keyword{
			Aliases:  []string{"container_rid"},
			Attr:     "NetNS",
			Example:  "container#0",
			Option:   "netns",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/netns"),
		},
	)
	return m
}
