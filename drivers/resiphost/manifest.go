package resiphost

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupIP, "host")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Add(
		keywords.Keyword{
			Option:   "ipname",
			Attr:     "IpName",
			Scopable: true,
			Example:  "1.2.3.4",
			Text:     "The DNS name or IP address of the ip resource. Can be different from one node to the other, in which case ``@nodename`` can be specified. This is most useful to specify a different ip when the service starts in DRP mode, where subnets are likely to be different than those of the production datacenter. With the amazon driver, the special ``<allocate>`` value tells the provisioner to assign a new private address.",
		},
		keywords.Keyword{
			Option:   "ipdev",
			Attr:     "IpDev",
			Scopable: true,
			Example:  "eth0",
			Required: true,
			Text:     "The interface name over which OpenSVC will try to stack the service ip. Can be different from one node to the other, in which case the ``@nodename`` can be specified. If the value is expressed as '<intf>:<alias>, the stacked interface index is forced to <alias> instead of the lowest free integer. If the value is expressed as <name>@<intf>, a macvtap interface named <name> is created and attached to <intf>.",
		},
		keywords.Keyword{
			Option:   "netmask",
			Attr:     "Netmask",
			Scopable: true,
			Example:  "24",
			Text:     "If an ip is already plumbed on the root interface (in which case the netmask is deduced from this ip). Mandatory if the interface is dedicated to the service (dummy interface are likely to be in this case). The format is either dotted or octal for IPv4, ex: 255.255.252.0 or 22, and octal for IPv6, ex: 64.",
		},
		keywords.Keyword{
			Option:       "gateway",
			Attr:         "Gateway",
			Scopable:     true,
			Text:         "A zone ip provisioning parameter used in the sysidcfg formatting. The format is decimal for IPv4, ex: 255.255.252.0, and octal for IPv6, ex: 64.",
			Provisioning: true,
		},
		keywords.Keyword{
			Option:    "wait_dns",
			Attr:      "WaitDNS",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      "Wait for the cluster DNS records associated to the resource to appear after a resource start and before the next resource can be started. This can be used for apps or containers that require the ip or ip name to be resolvable to provision or execute properly.",
		},
		keywords.Keyword{
			Option:   "dns_name_suffix",
			Attr:     "DNSNameSuffix",
			Scopable: true,
			Text:     "Add the value as a suffix to the DNS record name. The record created is thus formatted as ``<name>-<dns_name_suffix>.<app>.<managed zone>``.",
		},
		keywords.Keyword{
			Option:       "provisioner",
			Attr:         "Provisioner",
			Scopable:     true,
			Candidates:   []string{"collector", ""},
			Example:      "collector",
			Text:         "The IPAM driver to use to provision the ip.",
			Provisioning: true,
		},
		keywords.Keyword{
			Option:       "network",
			Attr:         "Network",
			Scopable:     true,
			Example:      "10.0.0.0/16",
			Text:         "The network, in dotted notation, from where the ip provisioner allocates. Also used by the docker ip driver to delete the network route if :kw:`del_net_route` is set to ``true``.",
			Provisioning: true,
		},
		keywords.Keyword{
			Option:    "dns_update",
			Attr:      "DNSUpdate",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      "Setting this parameter triggers a DNS update. The record created is formatted as ``<name>.<app>.<managed zone>``, unless dns_record_name is specified.",
		},
		keywords.Keyword{
			Option:    "check_carrier",
			Attr:      "CheckCarrier",
			Scopable:  true,
			Default:   "true",
			Converter: converters.Bool,
			Text:      "Activate the link carrier check. Set to false if ipdev is a backend bridge or switch.",
		},
		keywords.Keyword{
			Option:    "alias",
			Attr:      "Alias",
			Scopable:  true,
			Default:   "true",
			Converter: converters.Bool,
			Text:      "Use ip aliasing. Modern ip stack support multiple ip/mask per interface, so :kw:`alias` should be set to false when possible.",
		},
		keywords.Keyword{
			Option:    "expose",
			Attr:      "Expose",
			Scopable:  true,
			Converter: converters.List,
			Example:   "443/tcp:8443 53/udp",
			Text:      "A whitespace-separated list of ``<port>/<protocol>[:<host port>]`` describing socket services that mandate a SRV exposition. With <host_port> set, the ip.cni driver configures port mappings too.",
		},
	)
	return m
}
