package resiphost

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

	drvID = driver.NewID(driver.GroupIP, "host")
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
		resip.KeywordWaitDNS,
		keywords.Keyword{
			Aliases:  []string{"ipname"},
			Attr:     "Name",
			Example:  "1.2.3.4",
			Option:   "name",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Aliases:  []string{"ipdev"},
			Attr:     "Dev",
			Example:  "eth0",
			Option:   "dev",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dev"),
		},
		keywords.Keyword{
			Attr:     "Netmask",
			Example:  "24",
			Option:   "netmask",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/netmask"),
		},
		keywords.Keyword{
			Attr:         "Gateway",
			Option:       "gateway",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/gateway"),
		},
		keywords.Keyword{
			Attr:         "Network",
			Example:      "10.0.0.0/16",
			Option:       "network",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/network"),
		},
		keywords.Keyword{
			Attr:      "CheckCarrier",
			Converter: converters.Bool,
			Default:   "true",
			Option:    "check_carrier",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/check_carrier"),
		},
		keywords.Keyword{
			Attr:      "Alias",
			Converter: converters.Bool,
			Default:   "true",
			Option:    "alias",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/alias"),
		},
		keywords.Keyword{
			Attr:      "Expose",
			Converter: converters.List,
			Example:   "443/tcp:8443 53/udp",
			Option:    "expose",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/expose"),
		},
	)
	return m
}
