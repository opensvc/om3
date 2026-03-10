package resiphost

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

	drvID = driver.NewID(driver.GroupIP, "host")

	kws = []*keywords.Keyword{
		&resip.KeywordWaitDNS,
		{
			Aliases:  []string{"ipname"},
			Attr:     "Name",
			Example:  "1.2.3.4",
			Option:   "name",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Aliases:  []string{"ipdev"},
			Attr:     "Dev",
			Example:  "eth0",
			Option:   "dev",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dev"),
		},
		{
			Attr:     "Netmask",
			Example:  "24",
			Option:   "netmask",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/netmask"),
		},
		{
			Attr:         "Gateway",
			Option:       "gateway",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/gateway"),
		},
		{
			Attr:         "Network",
			Example:      "10.0.0.0/16",
			Option:       "network",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/network"),
		},
		{
			Attr:      "CheckCarrier",
			Converter: "bool",
			Default:   "true",
			Option:    "check_carrier",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/check_carrier"),
		},
		{
			Attr:      "Alias",
			Converter: "bool",
			Default:   "true",
			Option:    "alias",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/alias"),
		},
		{
			Attr:      "Expose",
			Converter: "list",
			Example:   "443/tcp:8443 53/udp",
			Option:    "expose",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/expose"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextObjectFQDN,
		manifest.ContextDNS,
	)
	m.AddKeywords(kws...)
	return m
}
