//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/core/actionresdeps"
	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/resip"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/netif"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/go-ping/ping"
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
		DNSNameSuffix string         `json:"dns_name_suffix"`
		NetNS         string         `json:"netns"`
		VLANTag       string         `json:"vlan_tag"`
		VLANMode      string         `json:"vlan_mode"`
		Mode          string         `json:"mode"`
		NSDev         string         `json:"nsdev"`
		MacAddr       string         `json:"mac_addr"`
		DelNetRoute   bool           `json:"del_net_route"`
		Name          string         `json:"name"`
		Dev           string         `json:"dev"`
		Netmask       string         `json:"netmask"`
		Gateway       string         `json:"gateway"`
		Network       string         `json:"network"`
		WaitDNS       *time.Duration `json:"wait_dns"`
		CheckCarrier  bool           `json:"check_carrier"`
		Alias         bool           `json:"alias"`
		Expose        []string       `json:"expose"`

		// cache
		_ipaddr net.IP
		_ipmask net.IPMask
		_ipnet  *net.IPNet
	}

	Addrs []net.Addr
)

var (
	ErrLinkInUse    = errors.New("link in use")
	ErrLinkNotFound = errors.New("link not found")
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) getResourceHostname(ctx context.Context) (string, error) {
	if r := t.GetObjectDriver().ResourceByID(t.NetNS); r == nil {
		return "", fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	} else if i, ok := r.(resource.GetHostnamer); !ok {
		return "", fmt.Errorf("resource %s pointed by the netns keyword does not expose a hostname", t.NetNS)
	} else {
		return i.GetHostname(), nil
	}
}

func (t *T) getNSPID(ctx context.Context) (int, error) {
	if r := t.GetObjectDriver().ResourceByID(t.NetNS); r == nil {
		return 0, fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	} else if i, ok := r.(resource.PIDer); !ok {
		return 0, fmt.Errorf("resource %s pointed by the netns keyword does not expose a pid", t.NetNS)
	} else {
		return i.PID(ctx), nil
	}
}

func (t *T) getNS(ctx context.Context) (ns.NetNS, error) {
	if r := t.GetObjectDriver().ResourceByID(t.NetNS); r == nil {
		return nil, fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	} else if i, ok := r.(resource.NetNSPather); !ok {
		return nil, fmt.Errorf("resource %s pointed by the netns keyword does not expose a netns path", t.NetNS)
	} else if path, err := i.NetNSPath(ctx); err != nil {
		return nil, err
	} else if path == "" {
		return nil, nil
	} else {
		return ns.GetNS(path)
	}
}

// StatusInfo implements resource.StatusInfoer
func (t *T) StatusInfo(ctx context.Context) map[string]interface{} {
	netmask, _ := t.ipmask().Size()
	data := make(map[string]interface{})
	data["expose"] = t.Expose
	data["ipaddr"] = t.ipaddr()
	data["dev"] = t.Dev
	data["netmask"] = netmask
	if hostname, _ := t.getResourceHostname(ctx); hostname != "" {
		if t.DNSNameSuffix != "" {
			hostname += t.DNSNameSuffix
		}
		data["hostname"] = hostname
	}
	return data
}

func (t *T) ActionResourceDeps() []actionresdeps.Dep {
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

func (t *T) startIP(ctx context.Context, netns ns.NetNS, guestDev string) error {
	var (
		ipnet *net.IPNet
		err   error
		isUp  bool
	)
	if err := netns.Do(func(_ ns.NetNS) error {
		ipnet, err = t.ipnetStrict()
		if err != nil {
			return err
		}
		if iface, err := net.InterfaceByName(guestDev); err != nil {
			return err
		} else if addrs, err := iface.Addrs(); err != nil {
			return err
		} else if Addrs(addrs).Has(ipnet.IP) {
			t.Log().Infof("%s is already up (on %s)", ipnet, guestDev)
			isUp = true
		}
		return nil
	}); err != nil {
		return err
	}
	if isUp {
		return nil
	}
	if ipnet != nil && ipnet.IP != nil && ipnet.IP.To4() == nil {
		if err := t.sysctlEnableIPV6In(guestDev, netns.Path()); err != nil {
			return err
		}
	}
	if err := t.addrAddIn(ipnet.String(), guestDev, netns.Path()); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.stopIP(netns, guestDev)
	})
	return nil
}

func (t *T) startRoutes(ctx context.Context, netns ns.NetNS, guestDev string) error {
	if t.Gateway == "" {
		if v, err := t.hasRouteDevIn("default", guestDev, netns.Path()); err != nil {
			return err
		} else if v {
			t.Log().Infof("route already added: default dev %s", guestDev)
			return nil
		}
		if err := t.routeAddDevIn("default", guestDev, netns.Path()); err != nil {
			return err
		}
	} else {
		if v, err := t.hasRouteViaIn("default", t.Gateway, netns.Path()); err != nil {
			return err
		} else if v {
			t.Log().Infof("route already added: default via %s", t.Gateway)
			return nil
		}
		if err := t.routeAddViaIn("default", t.Gateway, netns.Path()); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) startRoutesDel(ctx context.Context, netns ns.NetNS, guestDev string) error {
	if !t.DelNetRoute {
		return nil
	}
	if t.Network == "" {
		return nil
	}
	if v, err := t.hasLinkIn(guestDev, netns.Path()); err != nil {
		return err
	} else if !v {
		return nil
	}
	ones, _ := t.ipmask().Size()
	dest := fmt.Sprintf("%s/%d", t.Network, ones)
	if err := t.routeDelDevIn(dest, guestDev, netns.Path()); err != nil {
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

func (t *T) devMTU() (int, error) {
	iface, err := net.InterfaceByName(t.Dev)
	if err != nil {
		return 0, fmt.Errorf("%s mtu: %w", t.Dev, err)
	}
	return iface.MTU, nil
}

func (t *T) Status(ctx context.Context) status.T {
	var (
		err     error
		carrier bool
	)
	if t.Name == "" {
		t.StatusLog().Warn("name not set")
		return status.NotApplicable
	}
	if t.Dev == "" {
		t.StatusLog().Warn("dev not set")
		return status.NotApplicable
	}
	if _, err := t.netInterface(); err != nil {
		if t.Mode != "dedicated" {
			t.StatusLog().Error("%s", err)
			return status.Down
		}
	} else {
		if t.Mode == "dedicated" {
			return status.Down
		}
	}
	if t.CheckCarrier {
		if carrier, err = t.hasCarrier(); err == nil && carrier == false {
			t.StatusLog().Error("interface %s no-carrier.", t.Dev)
			return status.Down
		}
	}
	netns, err := t.getNS(ctx)
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

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	var dev string
	if t.NSDev != "" {
		dev = "@" + t.NSDev
	}
	ones, _ := t.ipmask().Size()
	return fmt.Sprintf("%s/%d%s in %s", t.ipaddr(), ones, dev, t.NetNS)
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) Abort(ctx context.Context) bool {
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

func (t *T) hasCarrier() (bool, error) {
	return netif.HasCarrier(t.Dev)
}

func (t *T) abortPing() bool {
	ip := t.ipaddr()
	pinger, err := ping.NewPinger(ip.String())
	if err != nil {
		t.Log().Errorf("abort? pinger init failed: %s", err)
		return true
	}
	pinger.Count = 5
	pinger.Timeout = 5 * time.Second
	pinger.Interval = time.Second
	t.Log().Infof("abort? checking %s availability with ping (5s)", ip)
	err = pinger.Run()
	if err != nil {
		t.Log().Warnf("abort? pinger run failed: %s", err)
		return false
	}
	if pinger.Statistics().PacketsRecv > 0 {
		t.Log().Errorf("abort! %s is alive", ip)
		return true
	}
	t.Log().Debugf("abort? %s is not alive", ip)
	return false
}

func (t *T) ipnet() *net.IPNet {
	if t._ipnet != nil {
		return t._ipnet
	}
	t._ipnet = t.getIPNet()
	return t._ipnet
}

func (t *T) ipaddr() net.IP {
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

func (t *T) getIPAddr() net.IP {
	switch {
	case naming.IsValidFQDN(t.Name) || hostname.IsValid(t.Name):
		var (
			l   []net.IP
			err error
		)
		l, err = net.LookupIP(t.Name)
		if err != nil {
			t.Log().Errorf("%s", err)
			return nil
		}
		n := len(l)
		switch n {
		case 0:
			t.Log().Errorf("name %s is unresolvable", t.Name)
		case 1:
			// ok
		default:
			t.Log().Debugf("name %s is resolvables to %d address. Using the first.", t.Name, n)
		}
		return l[0]
	default:
		return net.ParseIP(t.Name)
	}
}

func (t *T) netInterface() (*net.Interface, error) {
	return net.InterfaceByName(t.Dev)
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

func (t *T) arpAnnounce(dev string) error {
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

func (t *T) LinkTo() string {
	return t.NetNS
}

func (t *T) getLinkStringsIn(netns ns.NetNS) ([]string, error) {
	l := make([]string, 0)
	buff, err := t.linkListIn(netns.Path())
	if err != nil {
		return l, err
	}
	for _, line := range strings.Split(buff, "\n") {
		words := strings.Fields(line)
		if len(words) < 2 {
			continue
		}
		if !strings.HasSuffix(words[1], ":") {
			continue
		}
		dev := strings.TrimRight(words[1], ":")
		dev = strings.Split(dev, "@")[0]
		l = append(l, dev)
	}
	return l, nil
}

func (t *T) getAddrStringsIn(dev string, netns ns.NetNS) ([]string, error) {
	l := make([]string, 0)
	buff, err := t.addrListIn(dev, netns.Path())
	if err != nil {
		return l, err
	}
	for _, line := range strings.Split(buff, "\n") {
		words := strings.Fields(line)
		if len(words) < 2 {
			continue
		}
		if (words[0] != "inet") && (words[0] != "inet6") {
			continue
		}
		if slices.Contains(words, "mngtmpaddr") {
			// Discard addrs having the flag "mngtmpaddr" as they are autoconfigured.
			// See Privacy Extensions for SLAAC (RFC 4941)
			continue
		}
		if strings.HasPrefix(words[1], "fe80") {
			continue
		}
		l = append(l, words[1])
	}
	return l, nil
}

func (t *T) stopLinkIn(netns ns.NetNS, guestDev string) error {
	if guestDev == "" {
		// ip not found on any netns dev
		if t.NSDev != "" {
			guestDev = t.NSDev
		} else {
			return nil
		}
	}
	if v, err := t.hasLinkIn(guestDev, netns.Path()); err != nil {
		return err
	} else if !v {
		return ErrLinkNotFound
	}
	if addrs, err := t.getAddrStringsIn(guestDev, netns); err != nil {
		return err
	} else if len(addrs) > 0 {
		t.Log().Infof("preserve nsdev %s, in use by %s", guestDev, strings.Join(addrs, " "))
		return ErrLinkInUse
	}
	return t.linkDelIn(guestDev, netns.Path())
}

func (t *T) stopLink(dev string) error {
	if v, err := t.hasLink(dev); err != nil {
		return err
	} else if !v {
		return ErrLinkNotFound
	}
	return t.linkDel(dev)
}

func (t *T) stopIP(netns ns.NetNS, guestDev string) error {
	ipnet, err := t.ipnetStrict()
	if err != nil {
		return err
	}
	if guestDev == "" {
		t.Log().Infof("%s is already down (not found on any netns dev)", ipnet)
		return nil
	}
	return t.addrDelIn(ipnet.String(), guestDev, netns.Path())
}

func (t *T) ipnetStrict() (*net.IPNet, error) {
	ipnet := t.ipnet()
	if ipnet.Mask == nil {
		return nil, fmt.Errorf("ipnet definition error: %s/%s", t.ipaddr(), t.ipmask())
	}
	return ipnet, nil
}

func (t *T) curGuestDev(netns ns.NetNS) (string, error) {
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

func (t *T) newGuestDev(netns ns.NetNS) (string, error) {
	if t.NSDev != "" {
		return t.NSDev, nil
	}
	devs, err := t.getLinkStringsIn(netns)
	if err != nil {
		return "", err
	}
	i := 0
	for {
		name := fmt.Sprintf("eth%d", i)
		if !slices.Contains(devs, name) {
			return name, nil
		}
		i = i + 1
		if i > math.MaxUint16 {
			break
		}
	}
	return "", fmt.Errorf("can't find a free link name")
}

func (t *T) hasNSDev(netns ns.NetNS) bool {
	if t.NSDev == "" {
		return false
	}
	if v, err := t.hasLinkIn(t.NSDev, netns.Path()); err != nil {
		return false
	} else {
		return v
	}
}

func (t *T) guestDev(netns ns.NetNS) (string, error) {
	if dev, err := t.curGuestDev(netns); err != nil {
		return "", err
	} else if dev != "" {
		return dev, nil
	}
	return t.newGuestDev(netns)
}
