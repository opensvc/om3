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
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		manifest.ContextObjectPath,
		resip.KeywordWaitDNS,
		keywords.Keyword{
			Option:   "ipname",
			Attr:     "IPName",
			Scopable: true,
			Example:  "1.2.3.4",
			Text:     keywords.NewText(fs, "text/kw/ipname"),
		},
		keywords.Keyword{
			Option:   "ipdev",
			Attr:     "IPDev",
			Scopable: true,
			Example:  "eth0",
			Required: true,
			Text:     keywords.NewText(fs, "text/kw/ipdev"),
		},
		keywords.Keyword{
			Option:   "netmask",
			Attr:     "Netmask",
			Scopable: true,
			Example:  "24",
			Text:     keywords.NewText(fs, "text/kw/netmask"),
		},
		keywords.Keyword{
			Option:       "gateway",
			Attr:         "Gateway",
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/gateway"),
			Provisioning: true,
		},
		keywords.Keyword{
			Option:       "network",
			Attr:         "Network",
			Scopable:     true,
			Example:      "10.0.0.0/16",
			Text:         keywords.NewText(fs, "text/kw/network"),
			Provisioning: true,
		},
		keywords.Keyword{
			Option:    "check_carrier",
			Attr:      "CheckCarrier",
			Scopable:  true,
			Default:   "true",
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/ipname"),
		},
		keywords.Keyword{
			Option:    "alias",
			Attr:      "Alias",
			Scopable:  true,
			Default:   "true",
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/ipname"),
		},
		keywords.Keyword{
			Option:    "expose",
			Attr:      "Expose",
			Scopable:  true,
			Converter: converters.List,
			Example:   "443/tcp:8443 53/udp",
			Text:      keywords.NewText(fs, "text/kw/ipname"),
		},
	)
	return m
}
