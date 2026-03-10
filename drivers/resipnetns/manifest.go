//go:build linux

package resipnetns

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/resip"
	"github.com/opensvc/om3/v3/util/key"
)

var (
	//go:embed text
	fs embed.FS

	drvID    = driver.NewID(driver.GroupIP, "netns")
	altDrvID = driver.NewID(driver.GroupIP, "docker")

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
			Aliases:  []string{"container_rid"},
			Attr:     "NetNS",
			Example:  "container#0",
			Option:   "netns",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/netns"),
		},
		{
			Attr: "VLANTag",
			Depends: []keyop.T{
				{
					Key:   key.T{Option: "mode"},
					Op:    keyop.Equal,
					Value: "ovs",
				},
			},
			Example:  "44",
			Option:   "vlan_tag",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/vlan_tag"),
		},
		{
			Attr:       "VLANMode",
			Candidates: []string{"access", "native-tagged", "native-untagged"},
			Default:    "native-untagged",
			Depends: []keyop.T{
				{
					Key:   key.T{Option: "mode"},
					Op:    keyop.Equal,
					Value: "ovs",
				},
			},
			Example:  "access",
			Option:   "vlan_mode",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/vlan_mode"),
		},
		{
			Attr:       "Mode",
			Candidates: []string{"bridge", "dedicated", "macvlan", "ipvlan-l2", "ipvlan-l3", "ipvlan-l3s", "ovs"},
			Default:    "bridge",
			Example:    "access",
			Option:     "mode",
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/mode"),
		},
		{
			Attr:     "NSDev",
			Example:  "front",
			Option:   "nsdev",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/nsdev"),
		},
		{
			Attr:     "MacAddr",
			Example:  "ce:32:cc:ca:41:33",
			Option:   "macaddr",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/macaddr"),
		},
		{
			Attr:      "DelNetRoute",
			Converter: "bool",
			Default:   "false",
			Option:    "del_net_route",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/del_net_route"),
		},
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
	driver.Register(altDrvID, New)
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
