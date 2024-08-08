//go:build linux

package resipnetns

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/resip"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/key"
)

var (
	//go:embed text
	fs embed.FS

	drvID    = driver.NewID(driver.GroupIP, "netns")
	altDrvID = driver.NewID(driver.GroupIP, "docker")
)

func init() {
	driver.Register(drvID, New)
	driver.Register(altDrvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		manifest.ContextObjectPath,
		resip.KeywordWaitDNS,
		keywords.Keyword{
			Option:   "netns",
			Attr:     "NetNS",
			Scopable: true,
			Required: true,
			Aliases:  []string{"container_rid"},
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/netns"),
		},
		keywords.Keyword{
			Option:   "vlan_tag",
			Attr:     "VLANTag",
			Scopable: true,
			Depends: []keyop.T{
				{
					Key:   key.T{Option: "mode"},
					Op:    keyop.Equal,
					Value: "ovs",
				},
			},
			Example: "44",
			Text:    keywords.NewText(fs, "text/kw/vlan_tag"),
		},
		keywords.Keyword{
			Option:     "vlan_mode",
			Attr:       "VLANMode",
			Candidates: []string{"access", "native-tagged", "native-untagged"},
			Scopable:   true,
			Depends: []keyop.T{
				{
					Key:   key.T{Option: "mode"},
					Op:    keyop.Equal,
					Value: "ovs",
				},
			},
			Default: "native-untagged",
			Example: "access",
			Text:    keywords.NewText(fs, "text/kw/vlan_mode"),
		},
		keywords.Keyword{
			Option:     "mode",
			Attr:       "Mode",
			Candidates: []string{"bridge", "dedicated", "macvlan", "ipvlan-l2", "ipvlan-l3", "ovs"},
			Scopable:   true,
			Default:    "bridge",
			Example:    "access",
			Text:       keywords.NewText(fs, "text/kw/mode"),
		},
		keywords.Keyword{
			Option:   "nsdev",
			Attr:     "NSDev",
			Scopable: true,
			Example:  "front",
			Text:     keywords.NewText(fs, "text/kw/nsdev"),
		},
		keywords.Keyword{
			Option:   "macaddr",
			Attr:     "MacAddr",
			Scopable: true,
			Example:  "ce:32:cc:ca:41:33",
			Text:     keywords.NewText(fs, "text/kw/macaddr"),
		},
		keywords.Keyword{
			Option:    "del_net_route",
			Attr:      "DelNetRoute",
			Converter: converters.Bool,
			Scopable:  true,
			Default:   "false",
			Text:      keywords.NewText(fs, "text/kw/del_net_route"),
		},
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
			Text:      keywords.NewText(fs, "text/kw/check_carrier"),
		},
		keywords.Keyword{
			Option:    "alias",
			Attr:      "Alias",
			Scopable:  true,
			Default:   "true",
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/alias"),
		},
		keywords.Keyword{
			Option:    "expose",
			Attr:      "Expose",
			Scopable:  true,
			Converter: converters.List,
			Example:   "443/tcp:8443 53/udp",
			Text:      keywords.NewText(fs, "text/kw/expose"),
		},
	)
	return m
}
