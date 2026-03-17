//go:build linux

package resipcni

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/resip"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupIP, "cni")

	kws = []*keywords.Keyword{
		&resip.KeywordWaitDNS,
		{
			Attr:     "DNSNameSuffix",
			Example:  "-backup",
			Option:   "dns_name_suffix",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dns_name_suffix"),
		},
		{
			Attr:      "Expose",
			Converter: "list",
			Example:   "443/tcp:8443 53/udp",
			Option:    "expose",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/expose"),
		},
		{
			Attr:     "Network",
			Default:  "default",
			Example:  "mynet",
			Option:   "network",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/network"),
		},
		{
			Aliases:  []string{"ipdev"},
			Attr:     "NSDev",
			Example:  "front",
			Option:   "nsdev",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/nsdev"),
		},
		{
			Aliases:  []string{"container_rid"},
			Attr:     "NetNS",
			Example:  "container#0",
			Option:   "netns",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/netns"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		manifest.ContextCNIPlugins,
		manifest.ContextCNIConfig,
		manifest.ContextObjectID,
		manifest.ContextObjectPath,
		manifest.ContextObjectFQDN,
		manifest.ContextDNS,
	)
	m.AddKeywords(kws...)
	return m
}
