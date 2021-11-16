package resipnetns

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"opensvc.com/opensvc/core/actionresdeps"
	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/fqdn"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/netif"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/go-ping/ping"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
)

const (
	driverGroup = drivergroup.IP
	driverName  = "netns"

	tagNonRouted = "nonrouted"
	tagNoAction  = "noaction"
	tagDedicated = "dedicated"
)

type (
	T struct {
		resource.T

		// config
		NetNS         string   `json:"netns"`
		VLANTag       string   `json:"vlan_tag"`
		VLANMode      string   `json:"vlan_mode"`
		Mode          string   `json:"mode"`
		NSDev         string   `json:"nsdev"`
		MacAddr       string   `json:"mac_addr"`
		DelNetRoute   bool     `json:"del_net_route"`
		IpName        string   `json:"ipname"`
		IpDev         string   `json:"ipdev"`
		Netmask       string   `json:"netmask"`
		Gateway       string   `json:"gateway"`
		Network       string   `json:"network"`
		WaitDNS       bool     `json:"wait_dns"`
		DNSUpdate     bool     `json:"dns_update"`
		DNSNameSuffix string   `json:"dns_name_suffix"`
		Provisioner   string   `json:"provisioner"`
		CheckCarrier  bool     `json:"check_carrier"`
		Alias         bool     `json:"alias"`
		Expose        []string `json:"expose"`

		// cache
		_ipaddr net.IP
		_ipmask net.IPMask
		_ipnet  *net.IPNet
	}

	Addrs []net.Addr
)

func init() {
	resource.Register(driverGroup, driverName, New)
	resource.Register(driverGroup, "docker", New)
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "netns",
			Attr:     "NetNS",
			Scopable: true,
			Required: true,
			Aliases:  []string{"container_rid"},
			Example:  "container#0",
			Text:     "The resource id of the container to plumb the ip into.",
		},
		{
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
			Text:    "The VLAN tag the switch port will relay.",
		},
		{
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
			Text:    "The VLAN port mode.",
		},
		{
			Option:     "mode",
			Attr:       "Mode",
			Candidates: []string{"bridge", "dedicated", "macvlan", "ipvlan-l2", "ipvlan-l3", "ovs"},
			Scopable:   true,
			Default:    "bridge",
			Example:    "access",
			Text:       "The ip link mode. If ipdev is set to a bridge interface the mode defaults to bridge, else defaults to macvlan. ipvlan requires a 4.2+ kernel.",
		},
		{
			Option:      "nsdev",
			Attr:        "NSDev",
			Scopable:    true,
			DefaultText: "The first available eth<n>.",
			Example:     "front",
			Text:        "The interface name in the network namespace.",
		},
		{
			Option:   "macaddr",
			Attr:     "MacAddr",
			Scopable: true,
			Example:  "ce:32:cc:ca:41:33",
			Text:     "The hardware address to set on the interface in the network namespace.",
		},
		{
			Option:    "del_net_route",
			Attr:      "DelNetRoute",
			Converter: converters.Bool,
			Scopable:  true,
			Default:   "false",
			Text:      "Some docker ip configuration requires dropping the network route autoconfigured when installing the ip address. In this case set this parameter to true, and also set the network parameter.",
		},
		{
			Option:   "ipname",
			Attr:     "IpName",
			Scopable: true,
			Example:  "1.2.3.4",
			Text:     "The DNS name or IP address of the ip resource. Can be different from one node to the other, in which case ``@nodename`` can be specified. This is most useful to specify a different ip when the service starts in DRP mode, where subnets are likely to be different than those of the production datacenter. With the amazon driver, the special ``<allocate>`` value tells the provisioner to assign a new private address.",
		},
		{
			Option:   "ipdev",
			Attr:     "IpDev",
			Scopable: true,
			Example:  "eth0",
			Required: true,
			Text:     "The interface name over which OpenSVC will try to stack the service ip. Can be different from one node to the other, in which case the ``@nodename`` can be specified. If the value is expressed as '<intf>:<alias>, the stacked interface index is forced to <alias> instead of the lowest free integer. If the value is expressed as <name>@<intf>, a macvtap interface named <name> is created and attached to <intf>.",
		},
		{
			Option:   "netmask",
			Attr:     "Netmask",
			Scopable: true,
			Example:  "24",
			Text:     "If an ip is already plumbed on the root interface (in which case the netmask is deduced from this ip). Mandatory if the interface is dedicated to the service (dummy interface are likely to be in this case). The format is either dotted or octal for IPv4, ex: 255.255.252.0 or 22, and octal for IPv6, ex: 64.",
		},
		{
			Option:       "gateway",
			Attr:         "Gateway",
			Scopable:     true,
			Text:         "A zone ip provisioning parameter used in the sysidcfg formatting. The format is decimal for IPv4, ex: 255.255.252.0, and octal for IPv6, ex: 64.",
			Provisioning: true,
		},
		{
			Option:    "wait_dns",
			Attr:      "WaitDNS",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      "Wait for the cluster DNS records associated to the resource to appear after a resource start and before the next resource can be started. This can be used for apps or containers that require the ip or ip name to be resolvable to provision or execute properly.",
		},
		{
			Option:   "dns_name_suffix",
			Attr:     "DNSNameSuffix",
			Scopable: true,
			Text:     "Add the value as a suffix to the DNS record name. The record created is thus formatted as ``<name>-<dns_name_suffix>.<app>.<managed zone>``.",
		},
		{
			Option:       "provisioner",
			Attr:         "Provisioner",
			Scopable:     true,
			Candidates:   []string{"collector", ""},
			Example:      "collector",
			Text:         "The IPAM driver to use to provision the ip.",
			Provisioning: true,
		},
		{
			Option:       "network",
			Attr:         "Network",
			Scopable:     true,
			Example:      "10.0.0.0/16",
			Text:         "The network, in dotted notation, from where the ip provisioner allocates. Also used by the docker ip driver to delete the network route if :kw:`del_net_route` is set to ``true``.",
			Provisioning: true,
		},
		{
			Option:    "dns_update",
			Attr:      "DNSUpdate",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      "Setting this parameter triggers a DNS update. The record created is formatted as ``<name>.<app>.<managed zone>``, unless dns_record_name is specified.",
		},
		{
			Option:    "check_carrier",
			Attr:      "CheckCarrier",
			Scopable:  true,
			Default:   "true",
			Converter: converters.Bool,
			Text:      "Activate the link carrier check. Set to false if ipdev is a backend bridge or switch.",
		},
		{
			Option:    "alias",
			Attr:      "Alias",
			Scopable:  true,
			Default:   "true",
			Converter: converters.Bool,
			Text:      "Use ip aliasing. Modern ip stack support multiple ip/mask per interface, so :kw:`alias` should be set to false when possible.",
		},
		{
			Option:    "expose",
			Attr:      "Expose",
			Scopable:  true,
			Converter: converters.List,
			Example:   "443/tcp:8443 53/udp",
			Text:      "A whitespace-separated list of ``<port>/<protocol>[:<host port>]`` describing socket services that mandate a SRV exposition. With <host_port> set, the ip.cni driver configures port mappings too.",
		},
	}...)
	return m
}

func (t T) getNSPID() (int, error) {
	r := t.GetObjectDriver().ResourceByID(t.NetNS)
	if r == nil {
		return 0, fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	}
	i, ok := r.(resource.PIDer)
	if !ok {
		return 0, fmt.Errorf("resource %s pointed by the netns keyword does not expose a pid", t.NetNS)
	}
	return i.PID(), nil
}

func (t T) getNS() (ns.NetNS, error) {
	r := t.GetObjectDriver().ResourceByID(t.NetNS)
	if r == nil {
		return nil, fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	}
	i, ok := r.(resource.NetNSPather)
	if !ok {
		return nil, fmt.Errorf("resource %s pointed by the netns keyword does not expose a netns path", t.NetNS)
	}
	path, err := i.NetNSPath()
	if err != nil {
		return nil, err
	}
	return ns.GetNS(path)
}

func (t *T) StatusInfo() map[string]interface{} {
	netmask, _ := t.ipmask().Size()
	data := make(map[string]interface{})
	data["ipaddr"] = t.ipaddr()
	data["ipdev"] = t.IpDev
	data["netmask"] = netmask
	return data
}

func (t T) ActionResourceDeps() []actionresdeps.Dep {
	return []actionresdeps.Dep{
		{Action: "start", Kind: actionresdeps.KindSelect, A: t.RID(), B: t.NetNS},
		{Action: "start", Kind: actionresdeps.KindSelect, A: t.NetNS, B: t.RID()},
		{Action: "stop", Kind: actionresdeps.KindSelect, A: t.NetNS, B: t.RID()},
		{Action: "start", Kind: actionresdeps.KindAct, A: t.RID(), B: t.NetNS},
		{Action: "stop", Kind: actionresdeps.KindAct, A: t.NetNS, B: t.RID()},
	}
}

func (t *T) Start(ctx context.Context) error {
	if t.Tags.Has(tagDedicated) {
		t.Log().Info().Msgf("mode %s (via resource tag)", tagDedicated)
		return t.startDedicated(ctx)
	}
	t.Log().Info().Msgf("mode %s", t.Mode)
	switch t.Mode {
	case "bridge":
		return t.startBridge(ctx)
	case "dedicated":
		return t.startDedicated(ctx)
	case "ipvlan-l2":
		return t.startIPVLAN(ctx)
	case "ipvlan-l3":
		return t.startIPVLAN(ctx)
	case "macvlan":
		return t.startMACVLAN(ctx)
	case "ovs":
		return t.startOVS(ctx)
	default:
		return fmt.Errorf("unsupported mode: %s", t.Mode)
	}
}

func formatHostDevName(guestDev string, pid int) string {
	return fmt.Sprintf("v%spl%d", guestDev, pid)
}

func (t *T) stopVEthPair(hostDev string) error {
	if hostDev == "" {
		return nil
	}
	link, err := netlink.LinkByName(hostDev)
	if err != nil {
		t.Log().Debug().Str("dev", hostDev).Msg("host-side veth dev already deleted")
		return nil
	}
	t.Log().Info().Str("dev", hostDev).Msg("delete host-side veth dev")
	return netlink.LinkDel(link)
}

func (t *T) startVEthPair(ctx context.Context, netns ns.NetNS, hostDev, guestDev string, mtu int) error {
	hostNS, err := ns.GetCurrentNS()
	if err != nil {
		return err
	}
	if _, err := netlink.LinkByName(hostDev); err == nil {
		t.Log().Info().Str("dev", hostDev).Msg("host-side veth dev already exists")
		return nil
	}
	if err := netns.Do(func(_ ns.NetNS) error {
		t.Log().Info().
			Str("dev", hostDev).
			Str("peer", guestDev).
			Int("mtu", mtu).
			Msg("create veth pair")
		_, _, err := ip.SetupVethWithName(guestDev, hostDev, mtu, hostNS)
		return err
	}); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.stopVEthPair(hostDev)
	})
	return nil
}

func (t *T) startIP(ctx context.Context, netns ns.NetNS, guestDev string) error {
	if err := netns.Do(func(_ ns.NetNS) error {
		ipnet, err := t.ipnetStrict()
		if err != nil {
			return err
		}
		if iface, err := net.InterfaceByName(guestDev); err != nil {
			return err
		} else if addrs, err := iface.Addrs(); err != nil {
			return err
		} else if Addrs(addrs).Has(ipnet.IP) {
			t.Log().Info().Msgf("%s is already up (on %s)", ipnet, guestDev)
			return nil
		}
		t.Log().Info().Msgf("add %s to netns %s", ipnet, guestDev)
		if err := netif.AddAddr(guestDev, ipnet); err != nil {
			return errors.Wrapf(err, "in netns %s", guestDev)
		}
		actionrollback.Register(ctx, func() error {
			return t.stopIP(netns, guestDev)
		})
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (t *T) startRoutes(ctx context.Context, netns ns.NetNS, guestDev string) error {
	if err := netns.Do(func(_ ns.NetNS) error {
		_, defNet, _ := net.ParseCIDR("0.0.0.0/0")
		routes, err := netlink.RouteListFiltered(unix.AF_UNSPEC, &netlink.Route{Dst: nil}, netlink.RT_FILTER_DST)
		if err != nil {
			return errors.Wrap(err, "ip route list default")
		}
		if len(routes) == 0 {
			if t.Gateway == "" {
				dev, err := netlink.LinkByName(guestDev)
				if err != nil {
					return errors.Wrapf(err, "route add default dev %s", guestDev)
				}
				t.Log().Info().Msgf("route add default dev %s", guestDev)
				err = netlink.RouteAdd(&netlink.Route{
					LinkIndex: dev.Attrs().Index,
					Scope:     netlink.SCOPE_UNIVERSE,
					Dst:       defNet,
					Gw:        nil,
				})
				if err != nil {
					return errors.Wrapf(err, "route add default dev %s", guestDev)
				}
				return nil
			} else {
				t.Log().Info().Msgf("route add default via %s", t.Gateway)
				err = netlink.RouteAdd(&netlink.Route{
					LinkIndex: 0,
					Scope:     netlink.SCOPE_UNIVERSE,
					Dst:       defNet,
					Gw:        net.ParseIP(t.Gateway),
				})
				if err != nil {
					return errors.Wrapf(err, "route add default via %s", t.Gateway)
				}
				return nil
			}
		}
		curRoute := routes[0]
		if t.Gateway == "" {
			dev, err := netlink.LinkByName(guestDev)
			if err != nil {
				return errors.Wrapf(err, "route replace default dev %s", guestDev)
			}
			if curRoute.LinkIndex == dev.Attrs().Index {
				t.Log().Info().Msgf("route already added: default dev %s", guestDev)
				return nil
			}
			t.Log().Info().Msgf("route replace default dev %s", guestDev)
			curRoute.Dst = defNet
			curRoute.Gw = nil
			curRoute.LinkIndex = dev.Attrs().Index
			err = netlink.RouteReplace(&curRoute)
			if err != nil {
				return errors.Wrapf(err, "route replace default dev %s", guestDev)
			}
			return nil
		} else {
			if net.ParseIP(t.Gateway).Equal(curRoute.Gw) {
				t.Log().Info().Msgf("route already added: default via %s", t.Gateway)
				return nil
			}
			t.Log().Info().Msgf("route replace default via %s", t.Gateway)
			curRoute.Dst = defNet
			curRoute.Gw = net.ParseIP(t.Gateway)
			curRoute.LinkIndex = 0
			err = netlink.RouteReplace(&curRoute)
			if err != nil {
				return errors.Wrapf(err, "route replace default via %s", t.Gateway)
			}
			return nil
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (t *T) startRoutesDel(ctx context.Context, netns ns.NetNS, guestDev string) error {
	if !t.DelNetRoute {
		return nil
	}
	mask := t.ipnet().Mask
	n := &net.IPNet{
		IP:   t.ipaddr().Mask(mask),
		Mask: mask,
	}
	if err := netns.Do(func(_ ns.NetNS) error {
		dev, err := netlink.LinkByName(guestDev)
		if err != nil {
			return errors.Wrapf(err, "route del %s dev %s", n, guestDev)
		}
		route := &netlink.Route{
			LinkIndex: dev.Attrs().Index,
			Scope:     netlink.SCOPE_UNIVERSE,
			Dst:       n,
			Gw:        nil,
		}
		routes, err := netlink.RouteListFiltered(unix.AF_UNSPEC, route, netlink.RT_FILTER_DST|netlink.RT_FILTER_IIF)
		if err != nil {
			return errors.Wrapf(err, "ip route list %s dev %s", n, guestDev)
		}
		if len(routes) > 0 {
			for _, r := range routes {
				t.Log().Info().Msgf("route del %s dev %s", r.Dst, guestDev)
				err := netlink.RouteDel(&r)
				if err != nil {
					return errors.Wrapf(err, "route del %s dev %s", r.Dst, guestDev)
				}
				actionrollback.Register(ctx, func() error {
					return netns.Do(func(_ ns.NetNS) error {
						return netlink.RouteAdd(&r)
					})
				})
			}
		} else {
			t.Log().Info().Msgf("route already deleted: %s dev %s", n, guestDev)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (t *T) startARP(netns ns.NetNS, guestDev string) error {
	if err := netns.Do(func(_ ns.NetNS) error {
		return t.arpAnnounce(guestDev)
	}); err != nil {
		return err
	}
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if t.Tags.Has(tagDedicated) {
		t.Log().Info().Msgf("mode %s (via resource tag)", tagDedicated)
		return t.stopDedicated(ctx)
	}
	t.Log().Info().Msgf("mode %s", t.Mode)
	switch t.Mode {
	case "bridge":
		return t.stopBridge(ctx)
	case "dedicated":
		return t.stopDedicated(ctx)
	case "ipvlan-l2":
		return t.stopIPVLAN(ctx)
	case "ipvlan-l3":
		return t.stopIPVLAN(ctx)
	case "macvlan":
		return t.stopMACVLAN(ctx)
	case "ovs":
		return t.stopOVS(ctx)
	default:
		return fmt.Errorf("unsupported mode: %s", t.Mode)
	}
}

func (t T) devMTU() (int, error) {
	iface, err := net.InterfaceByName(t.IpDev)
	if err != nil {
		return 0, errors.Wrapf(err, "%s mtu", t.IpDev)
	}
	return iface.MTU, nil
}

func (t *T) Status(ctx context.Context) status.T {
	var (
		err     error
		carrier bool
	)
	if t.IpName == "" {
		t.StatusLog().Warn("ipname not set")
		return status.NotApplicable
	}
	if t.IpDev == "" {
		t.StatusLog().Warn("ipdev not set")
		return status.NotApplicable
	}
	if _, err := t.netInterface(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Down
	}
	if t.CheckCarrier {
		if carrier, err = t.hasCarrier(); err == nil && carrier == false {
			t.StatusLog().Error("interface %s no-carrier.", t.IpDev)
			return status.Down
		}
	}
	netns, err := t.getNS()
	if err != nil {
		t.StatusLog().Error("netns: %s", err)
		return status.Down
	}
	defer netns.Close()

	guestDev, err := t.curGuestDev(netns)
	if err != nil {
		t.StatusLog().Error("guest dev: %s", err)
		return status.Down
	}
	if guestDev == "" {
		return status.Down
	}
	return status.Up
}

func (t T) Label() string {
	return fmt.Sprintf("%s", t.ipaddr())
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t T) Abort(ctx context.Context) bool {
	if t.Tags.Has(tagNonRouted) || t.Tags.Has(tagNoAction) {
		return false // let start fail with an explicit error message
	}
	if t.ipaddr() == nil {
		return false // let start fail with an explicit error message
	}
	if initialStatus := t.Status(ctx); initialStatus == status.Up {
		return false // let start fail with an explicit error message
	}
	if t.abortPing() {
		return true
	}
	return false
}

func (t T) hasCarrier() (bool, error) {
	return netif.HasCarrier(t.IpDev)
}

func (t T) abortPing() bool {
	ip := t.ipaddr()
	pinger, err := ping.NewPinger(ip.String())
	if err != nil {
		t.Log().Error().Err(err).Msg("abort: ping")
		return true
	}
	pinger.Count = 5
	pinger.Timeout = 5 * time.Second
	pinger.Interval = time.Second
	t.Log().Info().Msgf("checking %s availability (5s)", ip)
	pinger.Run()
	return pinger.Statistics().PacketsRecv > 0
}

func (t T) ipnet() *net.IPNet {
	if t._ipnet != nil {
		return t._ipnet
	}
	t._ipnet = t.getIPNet()
	return t._ipnet
}

func (t T) ipaddr() net.IP {
	if t._ipaddr != nil {
		return t._ipaddr
	}
	t._ipaddr = t.getIPAddr()
	return t._ipaddr
}

func (t *T) ipmask() net.IPMask {
	if t._ipmask != nil {
		return t._ipmask
	}
	t._ipmask = t.getIPMask()
	return t._ipmask
}

func (t *T) getIPNet() *net.IPNet {
	return &net.IPNet{
		IP:   t.ipaddr(),
		Mask: t.ipmask(),
	}
}

func (t *T) getIPMask() net.IPMask {
	ip := t.ipaddr()
	bits := getIPBits(ip)
	if m, err := parseCIDRMask(t.Netmask, bits); err == nil {
		return m
	}
	if m, err := parseDottedMask(t.Netmask); err == nil {
		return m
	}
	// fallback to the mask of the first found ip on the intf
	if m, err := t.defaultMask(); err == nil {
		return m
	}
	return nil
}

func (t *T) defaultMask() (net.IPMask, error) {
	intf, err := t.netInterface()
	if err != nil {
		return nil, err
	}
	addrs, err := intf.Addrs()
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addr to guess mask from")
	}
	_, net, err := net.ParseCIDR(addrs[0].String())
	if err != nil {
		return nil, err
	}
	return net.Mask, nil
}

func (t T) getIPAddr() net.IP {
	switch {
	case fqdn.IsValid(t.IpName) || hostname.IsValid(t.IpName):
		var (
			l   []net.IP
			err error
		)
		l, err = net.LookupIP(t.IpName)
		if err != nil {
			t.Log().Error().Err(err)
			return nil
		}
		n := len(l)
		switch n {
		case 0:
			t.Log().Error().Msgf("ipname %s is unresolvable", t.IpName)
		case 1:
			// ok
		default:
			t.Log().Debug().Msgf("ipname %s is resolvables to %d address. Using the first.", t.IpName, n)
		}
		return l[0]
	default:
		return net.ParseIP(t.IpName)
	}
}

func (t T) netInterface() (*net.Interface, error) {
	return net.InterfaceByName(t.IpDev)
}

func (t Addrs) Has(ip net.IP) bool {
	for _, addr := range t {
		listIP, _, _ := net.ParseCIDR(addr.String())
		if ip.Equal(listIP) {
			return true
		}
	}
	return false
}

func parseCIDRMask(s string, bits int) (net.IPMask, error) {
	if bits == 0 {
		return nil, errors.New("invalid bits: 0")
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return nil, fmt.Errorf("invalid element in dotted mask: %s", err)
	}
	return net.CIDRMask(i, bits), nil
}

func parseDottedMask(s string) (net.IPMask, error) {
	m := []byte{}
	l := strings.Split(s, ".")
	if len(l) != 4 {
		return nil, errors.New("invalid number of elements in dotted mask")
	}
	for _, e := range l {
		i, err := strconv.Atoi(e)
		if err != nil {
			return nil, fmt.Errorf("invalid element in dotted mask: %s", err)
		}
		m = append(m, byte(i))
	}
	return m, nil
}

func ipv4MaskString(m []byte) string {
	if len(m) != 4 {
		panic("ipv4Mask: len must be 4 bytes")
	}

	return fmt.Sprintf("%d.%d.%d.%d", m[0], m[1], m[2], m[3])
}

func getIPBits(ip net.IP) (bits int) {
	switch {
	case ip.To4() != nil:
		bits = 32
	case ip.To16() != nil:
		bits = 128
	}
	return
}

func (t T) arpAnnounce(dev string) error {
	ip := t.ipaddr()
	if ip.IsLoopback() {
		t.Log().Debug().Msgf("skip arp announce on loopback address %s", ip)
		return nil
	}
	if ip.IsLinkLocalUnicast() {
		t.Log().Debug().Msgf("skip arp announce on link local unicast address %s", ip)
		return nil
	}
	if ip.To4() == nil {
		t.Log().Debug().Msgf("skip arp announce on non-ip4 address %s", ip)
		return nil
	}
	if i, err := net.InterfaceByName(dev); err == nil && i.Flags&net.FlagLoopback != 0 {
		t.Log().Debug().Msgf("skip arp announce on loopback interface %s", dev)
		return nil
	}
	t.Log().Info().Msgf("send gratuitous arp to announce %s over %s", t.ipaddr(), dev)
	if err := t.arpGratuitous(ip, dev); err != nil {
		return errors.Wrapf(err, "arping -i %s %s", dev, ip)
	}
	return nil
}

func (t T) LinkTo() string {
	return t.NetNS
}

func (t *T) stopLink(netns ns.NetNS, guestDev string) error {
	if guestDev == "" {
		// ip not found on any netns dev
		return nil
	}
	t.Log().Info().Msgf("delete netns link %s", guestDev)
	if err := netns.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(guestDev)
		if err != nil {
			return err
		}
		return netlink.LinkDel(link)
	}); err != nil {
		return err
	}
	return nil
}

func (t *T) stopIP(netns ns.NetNS, guestDev string) error {
	if err := netns.Do(func(_ ns.NetNS) error {
		ipnet, err := t.ipnetStrict()
		if err != nil {
			return err
		}
		if guestDev == "" {
			t.Log().Info().Msgf("%s is already down (not found on any netns dev)", ipnet)
			return nil
		}
		t.Log().Info().Msgf("delete %s from %s", ipnet, guestDev)
		return netif.DelAddr(guestDev, ipnet)
	}); err != nil {
		return err
	}
	return nil
}

func (t *T) ipnetStrict() (*net.IPNet, error) {
	ipnet := t.ipnet()
	if ipnet.Mask == nil {
		return nil, fmt.Errorf("ipnet definition error: %s/%s", t.ipaddr(), t.ipmask())
	}
	return ipnet, nil
}

func (t T) curGuestDev(netns ns.NetNS) (string, error) {
	ref := t.ipnet()
	s := ""
	if err := netns.Do(func(_ ns.NetNS) error {
		var err error
		s, err = netif.InterfaceNameByIP(ref)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", err
	}
	return s, nil
}

func (t T) newGuestDev(netns ns.NetNS) (string, error) {
	var (
		name string
		i    int
	)
	if t.NSDev != "" {
		return t.NSDev, nil
	}
	err := netns.Do(func(_ ns.NetNS) error {
		for {
			name = fmt.Sprintf("eth%d", i)
			_, err := netlink.LinkByName(name)
			if err != nil {
				return nil
			}
			i = i + 1
		}
		return nil
	})
	return name, err
}

func (t T) guestDev(netns ns.NetNS) (string, error) {
	if dev, err := t.curGuestDev(netns); err != nil {
		return "", err
	} else if dev != "" {
		return dev, nil
	}
	return t.newGuestDev(netns)
}
