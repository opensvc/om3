//go:build linux

package resipnetns

import (
	"context"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/core/actionresdeps"
	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/resip"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/netif"
	"github.com/rs/zerolog"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/go-ping/ping"
	"github.com/vishvananda/netlink"
)

const (
	tagNonRouted = "nonrouted"
	tagDedicated = "dedicated"
)

type (
	T struct {
		resource.T

		Path       naming.Path
		ObjectFQDN string
		DNS        []string

		// config
		NetNS        string         `json:"netns"`
		VLANTag      string         `json:"vlan_tag"`
		VLANMode     string         `json:"vlan_mode"`
		Mode         string         `json:"mode"`
		NSDev        string         `json:"nsdev"`
		MacAddr      string         `json:"mac_addr"`
		DelNetRoute  bool           `json:"del_net_route"`
		IPName       string         `json:"ipname"`
		IPDev        string         `json:"ipdev"`
		Netmask      string         `json:"netmask"`
		Gateway      string         `json:"gateway"`
		Network      string         `json:"network"`
		WaitDNS      *time.Duration `json:"wait_dns"`
		CheckCarrier bool           `json:"check_carrier"`
		Alias        bool           `json:"alias"`
		Expose       []string       `json:"expose"`

		// cache
		_ipaddr net.IP
		_ipmask net.IPMask
		_ipnet  *net.IPNet
	}

	Addrs []net.Addr
)

func New() resource.Driver {
	t := &T{}
	return t
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
	if path == "" {
		return nil, nil
	}
	return ns.GetNS(path)
}

func (t *T) StatusInfo() map[string]interface{} {
	netmask, _ := t.ipmask().Size()
	data := make(map[string]interface{})
	data["expose"] = t.Expose
	data["ipaddr"] = t.ipaddr()
	data["ipdev"] = t.IPDev
	data["netmask"] = netmask
	return data
}

func (t T) ActionResourceDeps() []actionresdeps.Dep {
	return []actionresdeps.Dep{
		{Action: "start", A: t.RID(), B: t.NetNS},
		{Action: "start", A: t.NetNS, B: t.RID()},
		{Action: "stop", A: t.NetNS, B: t.RID()},
	}
}

func (t *T) Start(ctx context.Context) error {
	if err := t.startMode(ctx); err != nil {
		return err
	}
	if err := resip.WaitDNSRecord(ctx, t.WaitDNS, t.ObjectFQDN, t.DNS); err != nil {
		return err
	}
	return nil
}

func (t *T) startMode(ctx context.Context) error {
	if t.Tags.Has(tagDedicated) {
		return t.startDedicated(ctx)
	}
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
		t.Log().Debugf("host-side veth dev '%s' already deleted", hostDev)
		return nil
	}
	t.Log().Infof("delete host-side veth dev '%s'", hostDev)
	return netlink.LinkDel(link)
}

func (t *T) startVEthPair(ctx context.Context, netns ns.NetNS, hostDev, guestDev string, mtu int) error {
	hostNS, err := ns.GetCurrentNS()
	if err != nil {
		return err
	}
	if _, err := netlink.LinkByName(hostDev); err == nil {
		t.Log().Infof("host-side veth dev '%s' already exists", hostDev)
		return nil
	}
	if err := netns.Do(func(_ ns.NetNS) error {
		t.Log().
			Attr("dev", hostDev).
			Attr("peer", guestDev).
			Attr("mtu", mtu).
			Infof("create veth pair: %s-%s mtu %d", hostDev, guestDev, mtu)
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
		t.Log().Infof("nsenter --net=%s sysctl -w net.ipv6.conf.%s.disable_ipv6=0", netns.Path(), guestDev)
		cmd := command.New(
			command.WithName("sysctl"),
			command.WithArgs([]string{"-w", fmt.Sprintf("net.ipv6.conf.%s.disable_ipv6=0", guestDev)}),
			command.WithLogger(t.Log()),
			command.WithStdoutLogLevel(zerolog.InfoLevel),
			command.WithStderrLogLevel(zerolog.WarnLevel),
		)
		if err := cmd.Run(); err != nil {
			return err
		}
		if iface, err := net.InterfaceByName(guestDev); err != nil {
			return err
		} else if addrs, err := iface.Addrs(); err != nil {
			return err
		} else if Addrs(addrs).Has(ipnet.IP) {
			t.Log().Infof("%s is already up (on %s)", ipnet, guestDev)
			return nil
		}
		t.Log().Infof("add %s to %s", ipnet, guestDev)
		if err := netif.AddAddr(guestDev, ipnet); err != nil {
			return fmt.Errorf("in netns %s: %w", guestDev, err)
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
			return fmt.Errorf("ip route list default: %w", err)
		}
		if len(routes) == 0 {
			if t.Gateway == "" {
				dev, err := netlink.LinkByName(guestDev)
				if err != nil {
					return fmt.Errorf("route add default dev %s: %w", guestDev, err)
				}
				t.Log().Infof("route add default dev %s", guestDev)
				err = netlink.RouteAdd(&netlink.Route{
					LinkIndex: dev.Attrs().Index,
					Scope:     netlink.SCOPE_UNIVERSE,
					Dst:       defNet,
					Gw:        nil,
				})
				if err != nil {
					return fmt.Errorf("route add default dev %s: %w", guestDev, err)
				}
				return nil
			} else {
				t.Log().Infof("route add default via %s", t.Gateway)
				err = netlink.RouteAdd(&netlink.Route{
					LinkIndex: 0,
					Scope:     netlink.SCOPE_UNIVERSE,
					Dst:       defNet,
					Gw:        net.ParseIP(t.Gateway),
				})
				if err != nil {
					return fmt.Errorf("route add default via %s: %w", t.Gateway, err)
				}
				return nil
			}
		}
		curRoute := routes[0]
		if t.Gateway == "" {
			dev, err := netlink.LinkByName(guestDev)
			if err != nil {
				return fmt.Errorf("route replace default dev %s: %w", guestDev, err)
			}
			if curRoute.LinkIndex == dev.Attrs().Index {
				t.Log().Infof("route already added: default dev %s", guestDev)
				return nil
			}
			t.Log().Infof("route replace default dev %s", guestDev)
			curRoute.Dst = defNet
			curRoute.Gw = nil
			curRoute.LinkIndex = dev.Attrs().Index
			err = netlink.RouteReplace(&curRoute)
			if err != nil {
				return fmt.Errorf("route replace default dev %s: %w", guestDev, err)
			}
			return nil
		} else {
			if net.ParseIP(t.Gateway).Equal(curRoute.Gw) {
				t.Log().Infof("route already added: default via %s", t.Gateway)
				return nil
			}
			t.Log().Infof("route replace default via %s", t.Gateway)
			curRoute.Dst = defNet
			curRoute.Gw = net.ParseIP(t.Gateway)
			curRoute.LinkIndex = 0
			err = netlink.RouteReplace(&curRoute)
			if err != nil {
				return fmt.Errorf("route replace default via %s: %w", t.Gateway, err)
			}
			return nil
		}
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
			return fmt.Errorf("route del %s dev %s: %w", n, guestDev, err)
		}
		route := &netlink.Route{
			LinkIndex: dev.Attrs().Index,
			Scope:     netlink.SCOPE_UNIVERSE,
			Dst:       n,
			Gw:        nil,
		}
		routes, err := netlink.RouteListFiltered(unix.AF_UNSPEC, route, netlink.RT_FILTER_DST|netlink.RT_FILTER_IIF)
		if err != nil {
			return fmt.Errorf("ip route list %s dev %s: %w", n, guestDev, err)
		}
		if len(routes) > 0 {
			for _, r := range routes {
				t.Log().Infof("route del %s dev %s", r.Dst, guestDev)
				err := netlink.RouteDel(&r)
				if err != nil {
					return fmt.Errorf("route del %s dev %s: %w", r.Dst, guestDev, err)
				}
				actionrollback.Register(ctx, func() error {
					return netns.Do(func(_ ns.NetNS) error {
						return netlink.RouteAdd(&r)
					})
				})
			}
		} else {
			t.Log().Infof("route already deleted: %s dev %s", n, guestDev)
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
		return t.stopDedicated(ctx)
	}
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
	iface, err := net.InterfaceByName(t.IPDev)
	if err != nil {
		return 0, fmt.Errorf("%s mtu: %w", t.IPDev, err)
	}
	return iface.MTU, nil
}

func (t *T) Status(ctx context.Context) status.T {
	var (
		err     error
		carrier bool
	)
	if t.IPName == "" {
		t.StatusLog().Warn("ipname not set")
		return status.NotApplicable
	}
	if t.IPDev == "" {
		t.StatusLog().Warn("ipdev not set")
		return status.NotApplicable
	}
	if _, err := t.netInterface(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Down
	}
	if t.CheckCarrier {
		if carrier, err = t.hasCarrier(); err == nil && carrier == false {
			t.StatusLog().Error("interface %s no-carrier.", t.IPDev)
			return status.Down
		}
	}
	netns, err := t.getNS()
	if err != nil {
		t.StatusLog().Error("netns: %s", err)
		return status.Down
	}
	if netns == nil {
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
	if t.Tags.Has(tagNonRouted) || t.IsActionDisabled() {
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
	return netif.HasCarrier(t.IPDev)
}

func (t T) abortPing() bool {
	ip := t.ipaddr()
	pinger, err := ping.NewPinger(ip.String())
	if err != nil {
		t.Log().Errorf("abort: ping: %s", err)
		return true
	}
	pinger.Count = 5
	pinger.Timeout = 5 * time.Second
	pinger.Interval = time.Second
	t.Log().Infof("checking %s availability (5s)", ip)
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
	case naming.IsValidFQDN(t.IPName) || hostname.IsValid(t.IPName):
		var (
			l   []net.IP
			err error
		)
		l, err = net.LookupIP(t.IPName)
		if err != nil {
			t.Log().Errorf("%s", err)
			return nil
		}
		n := len(l)
		switch n {
		case 0:
			t.Log().Errorf("ipname %s is unresolvable", t.IPName)
		case 1:
			// ok
		default:
			t.Log().Debugf("ipname %s is resolvables to %d address. Using the first.", t.IPName, n)
		}
		return l[0]
	default:
		return net.ParseIP(t.IPName)
	}
}

func (t T) netInterface() (*net.Interface, error) {
	return net.InterfaceByName(t.IPDev)
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
		return nil, fmt.Errorf("invalid bits: 0")
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
		return nil, fmt.Errorf("invalid number of elements in dotted mask")
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
		t.Log().Debugf("skip arp announce on loopback address %s", ip)
		return nil
	}
	if ip.IsLinkLocalUnicast() {
		t.Log().Debugf("skip arp announce on link local unicast address %s", ip)
		return nil
	}
	if ip.To4() == nil {
		t.Log().Debugf("skip arp announce on non-ip4 address %s", ip)
		return nil
	}
	if i, err := net.InterfaceByName(dev); err == nil && i.Flags&net.FlagLoopback != 0 {
		t.Log().Debugf("skip arp announce on loopback interface %s", dev)
		return nil
	}
	t.Log().Infof("send gratuitous arp to announce %s over %s", t.ipaddr(), dev)
	if err := t.arpGratuitous(ip, dev); err != nil {
		return fmt.Errorf("arping -i %s %s: %w", dev, ip, err)
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
	t.Log().Infof("delete netns link %s", guestDev)
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
			t.Log().Infof("%s is already down (not found on any netns dev)", ipnet)
			return nil
		}
		t.Log().Infof("delete %s from %s", ipnet, guestDev)
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
	if netns == nil {
		return "", fmt.Errorf("can't get current guest dev from nil netns")
	}
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
			if i > math.MaxUint16 {
				return fmt.Errorf("can't find available link at %s", name)
			}
		}
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
