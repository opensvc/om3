package resiphost

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/drivers/resip"
	"github.com/opensvc/om3/v3/util/duration"
	"github.com/opensvc/om3/v3/util/getaddr"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/netif"
	"github.com/opensvc/om3/v3/util/ping"
)

const (
	tagNonRouted = "nonrouted"
	maxIPAddrAge = 19 * time.Minute
)

type (
	T struct {
		resource.T
		resource.Restart

		Path       naming.Path
		ObjectFQDN string
		DNS        []string

		// config
		Name         string         `json:"name"`
		Dev          string         `json:"dev"`
		Netmask      string         `json:"netmask"`
		Network      string         `json:"network"`
		Addr         string         `json:"addr"`
		Gateway      string         `json:"gateway"`
		Provisioner  string         `json:"provisioner"`
		CheckCarrier bool           `json:"check_carrier"`
		Alias        bool           `json:"alias"`
		Expose       []string       `json:"expose"`
		WaitDNS      *time.Duration `json:"wait_dns"`

		// cache
		_ipaddr    net.IP
		_ipaddrAge time.Duration
		_ipmask    net.IPMask
		_ipnet     *net.IPNet
		_alloc     *resip.Allocation

		// netErr says why the network the address is drawn from does not
		// tell the interface or the prefix length of the address on this
		// node, reported when they are needed rather than failing every
		// load of the object.
		netErr error
	}

	Addrs []net.Addr
)

func New() resource.Driver {
	t := &T{}
	return t
}

// alloc is the address this resource draws from the network its network
// keyword names, when it names no address of its own.
func (t *T) alloc() *resip.Allocation {
	if t._alloc == nil {
		rid := t.RID()
		if t.perInstance() {
			// The reservation of this instance, apart from the ones of the
			// instances of the other nodes.
			rid += "@" + hostname.Hostname()
		}
		t._alloc = &resip.Allocation{Network: t.Network, Path: t.Path, RID: rid, Log: t.Log()}
	}
	return t._alloc
}

// perInstance says each instance draws an address of its own, which is the
// case of a resource not shared of a flex object: its instances run at once,
// and an address they all brought up would be one address on several nodes.
//
// A failover object runs one instance at a time, which takes the address
// along when it moves. A shared resource of a flex object is one address for
// all its instances, should that be the point.
func (t *T) perInstance() bool {
	if t.Shared {
		return false
	}
	o, ok := t.GetObject().(interface{ Topology() topology.T })
	return ok && o.Topology() == topology.Flex
}

// hasOwnAddr says the configuration records an address for this instance
// under its own key, rather than one the evaluation fell back on.
func (t *T) hasOwnAddr() bool {
	o, ok := t.GetObject().(object.Configurer)
	return ok && o.Config().HasKey(t.addrKey())
}

// addrKey is where the address drawn is recorded: addr, the same on every
// node, or addr@<node>, the address root chose for this instance when each
// instance draws its own.
func (t *T) addrKey() key.T {
	if t.perInstance() {
		return key.New(t.RID(), "addr@"+hostname.Hostname())
	}
	return key.New(t.RID(), "addr")
}

// isAllocated says the address is drawn from an om network: the
// configuration names none, and names a network.
func (t *T) isAllocated() bool {
	return t.Name == "" && t.Network != ""
}

// Configure fills from the network what the configuration did not say: the
// interface of this node the address is configured on, and the prefix length
// it is configured with. They are the network's to know, so naming the
// network is enough, and an explicit value always wins.
func (t *T) Configure() error {
	nw, err := t.alloc().Resolve()
	if err != nil {
		return err
	}
	if nw == nil {
		return nil
	}
	if t.Addr != "" && t.perInstance() && !t.hasOwnAddr() {
		// The address of all the instances, as a failover object records
		// it, read here for want of one chosen for this instance: each
		// instance of a flex resource draws its own.
		t.Addr = ""
	}
	if t.Dev == "" {
		t.Dev, t.netErr = networkDev(nw)
	}
	if t.Netmask == "" {
		if i, ok := nw.(network.Netmasker); ok {
			if n, err := i.Netmask(); err == nil {
				t.Netmask = fmt.Sprint(n)
			} else if t.netErr == nil {
				t.netErr = err
			}
		} else if i, err := t.alloc().Allocator(); err == nil && i != nil && i.Range != nil {
			ones, _ := i.Range.Mask.Size()
			t.Netmask = fmt.Sprint(ones)
		}
	}
	return nil
}

// networkDev returns the interface of this node the addresses of a network are
// configured on: the interface of the node on the segment of a lan network,
// the bridge of a bridge network.
func networkDev(nw network.Networker) (string, error) {
	switch i := nw.(type) {
	case network.HostDever:
		return i.HostDev()
	case interface{ BackendDevName() string }:
		if dev := i.BackendDevName(); dev != "" {
			return dev, nil
		}
	}
	return "", fmt.Errorf("network %s names no interface of this node to configure the address on: set dev", nw.Name())
}

// addrName is how the messages name the address: the configured name, or the
// address drawn from the network.
func (t *T) addrName() string {
	if t.Name != "" {
		return t.Name
	}
	if ip := t.ipaddr(); ip != nil {
		return ip.String()
	}
	return "the address of network " + t.Network
}

// checkDev returns why the address has no interface, or no prefix length, to
// be configured with.
func (t *T) checkDev() error {
	if t.netErr != nil {
		return t.netErr
	}
	if t.Dev == "" {
		return fmt.Errorf("dev is not set")
	}
	return nil
}

// StatusInfo implements resource.StatusInfoer
func (t *T) StatusInfo(_ context.Context) map[string]interface{} {
	netmask, _ := t.ipmask().Size()
	data := make(map[string]interface{})
	data["expose"] = t.Expose
	data["ipaddr"] = t.ipaddr()
	data["dev"] = t.Dev
	data["netmask"] = netmask
	return data
}

func (t *T) getDevAndLabel() (string, string, error) {
	dev, idx := resip.SplitDevLabel(t.Dev)
	label := ""
	if idx == "" {
		if !t.Alias {
			// ip#0.dev = eth0
			// ip#0.alias = false
			// => allocate a label
			if s, err := resip.AllocateDevLabel(dev); err != nil {
				return "", "", err
			} else {
				label = s
			}
		}
	} else {
		// ip#0.dev = eth0:0
		label = t.Dev
	}
	return dev, label, nil
}

func (t *T) Start(ctx context.Context) error {
	if err := t.checkDev(); err != nil {
		return err
	}
	if t.isAllocated() {
		ip, previous, err := t.reserve(ctx)
		if err != nil {
			return err
		}
		t._ipaddr, t._ipnet, t._ipmask = ip, nil, nil
		if err := t.dropPrevious(previous); err != nil {
			return err
		}
	}
	if initialStatus := t.statusWithIPAddrCacheTrust(ctx); initialStatus == status.Up {
		t.Log().Infof("%s is already up on %s", t.addrName(), t.Dev)
		return nil
	}
	if t._ipaddrAge > maxIPAddrAge {
		return fmt.Errorf("ip %s lookup issue, cache expired (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
	} else if t._ipaddrAge > 0 {
		t.Log().Warnf("ip %s lookup issue, cache valid (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
	}
	dev, label, err := t.getDevAndLabel()
	if err != nil {
		return err
	}
	if err := t.start(dev, label); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.stopAddr(ctx, dev)
	})
	if err := t.arpAnnounce(dev); err != nil {
		return err
	}
	if err := resip.WaitDNSRecord(ctx, t.WaitDNS, t.ObjectFQDN, t.DNS); err != nil {
		return err
	}
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if t._ipaddrAge > maxIPAddrAge {
		return fmt.Errorf("ip %s lookup issue, cache expired (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
	} else if t._ipaddrAge > 0 {
		t.Log().Warnf("ip %s lookup issue, cache valid (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
	}
	dev, _ := resip.SplitDevLabel(t.Dev)
	if err := t.stopAddr(ctx, dev); err != nil {
		return err
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	s := t.statusWithIPAddrCacheTrust(ctx)
	if s == status.Up && t._ipaddrAge > 0 {
		return status.Warn
	}
	if dups := t.duplicates(); len(dups) > 0 {
		t.StatusLog().Warn("%s is also held by %s: one of them is to be given another address", t.ipaddr(), strings.Join(dups, ", "))
		if s == status.Up {
			return status.Warn
		}
	}
	return s
}

// duplicates returns the resources of the other nodes the peer records say
// hold the address of this one, as "<key> on <node>".
//
// The address of a failover resource is the same on every node, a stopped
// instance reporting it too, which is no duplicate. Another resource holding
// it, or another instance of a resource drawing per instance, is: two halves
// of a cluster not hearing each other draw from the same range blind to each
// other, and the duplicate is for an administrator to clean up.
func (t *T) duplicates() []string {
	ip := t.ipaddr()
	if ip == nil || t.Network == "" {
		return nil
	}
	records, err := ipam.ReadPeerRecords(ipam.PeerDir(t.Network))
	if err != nil {
		return nil
	}
	key := ipam.Key(t.Path, t.RID())
	l := make([]string, 0)
	for _, record := range records {
		if !record.IP.Equal(ip) || record.Node == hostname.Hostname() {
			continue
		}
		if record.Key == key && !t.perInstance() {
			continue
		}
		l = append(l, fmt.Sprintf("%s on %s", record.Key, record.Node))
	}
	return l
}

func (t *T) statusWithIPAddrCacheTrust(ctx context.Context) status.T {
	switch {
	case t.isAllocated():
		if t.ipaddr() == nil {
			t.StatusLog().Info("no address drawn from network %s yet: a start draws one", t.Network)
			return status.Down
		}
	case t.Name == "":
		t.StatusLog().Warn("name not set")
		return status.NotApplicable
	}
	if err := t.checkDev(); err != nil {
		t.StatusLog().Warn("%s", err)
		return status.NotApplicable
	}
	dev, _ := resip.SplitDevLabel(t.Dev)
	if t.statusOfCarrier(ctx, dev) == status.Down {
		return status.Down
	}
	return t.statusOfAddr(ctx, dev)
}

func (t *T) statusOfCarrier(ctx context.Context, dev string) status.T {
	if !t.CheckCarrier {
		return status.NotApplicable
	}
	if carrier, err := t.hasCarrier(); err == nil && carrier == false {
		t.StatusLog().Error("interface %s no-carrier.", dev)
		return status.Down
	} else if err != nil {
		t.StatusLog().Warn("carrier: %s", err)
		return status.Undef
	}
	return status.Up
}

func (t *T) statusOfAddr(ctx context.Context, dev string) status.T {
	var (
		i     *net.Interface
		err   error
		addrs Addrs
	)
	if t.Name == "" && !t.isAllocated() {
		return status.NotApplicable
	}
	ip := t.ipaddr()
	if ip == nil && t.isAllocated() {
		return status.Down
	} else if ip == nil {
		t.StatusLog().Error("ip %s lookup issue, cache miss", t.Name)
		return status.Undef
	} else if t._ipaddrAge > maxIPAddrAge {
		t.StatusLog().Error("ip %s lookup issue, cache expired (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
		return status.Warn
	} else if t._ipaddrAge > 0 {
		t.StatusLog().Warn("ip %s lookup issue, cache valid (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
	}
	if i, err = net.InterfaceByName(dev); err != nil {
		if fmt.Sprint(err.(*net.OpError).Unwrap()) == "no such network interface" {
			t.StatusLog().Warn("interface %s not found", dev)
		} else {
			t.StatusLog().Error("%s", err)
		}
		return status.Down
	}
	if addrs, err = i.Addrs(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Down
	}
	if !addrs.Has(ip) {
		t.Log().Tracef("ip not found on intf")
		return status.Down
	}
	return status.Up
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

// UnprovisionStop stops the resource, and releases the address drawn from
// the network, which a stop keeps: the address is the service's, which its
// clients know it by, until the service is unprovisioned. Every node releases
// its reservation, and the leader removes the address from the configuration,
// once for all of them.
func (t *T) UnprovisionStop(ctx context.Context, leader bool) error {
	if err := t.Stop(ctx); err != nil {
		return err
	}
	if t.Network == "" || t.IsUnprovisionDisabled() {
		// A resource set to keep what it was given at unprovision keeps
		// its address too.
		return nil
	}
	if err := t.alloc().Free(); err != nil {
		return err
	}
	if t.Addr == "" || t.Name != "" || t.perInstance() {
		// An address of one instance is not recorded: an addr@<node> is
		// one root chose, and stays.
		return nil
	}
	if !leader {
		// The address of all the instances, which the leader removes.
		return nil
	}
	return t.setAddr(ctx, "")
}

// reserve returns the address of the resource, reserved on this node: the one
// the configuration says it drew, or else one drawn now and written to the
// configuration. It also returns the address the resource gave up for it,
// nil when it gave none up.
//
// The configuration is what every node holds, whatever became of the node
// that drew the address: a node the service fails over to after the others
// crashed reads it there, rather than draw again from a cluster that forgot
// them.
//
// A configuration with no address while this node holds one for the resource
// had it unset, since om writes it as it draws and unsets it as it releases.
// Unsetting it is asking for another address, so the one held is given up
// rather than drawn again, which the draw would do, keyed as it is on the
// resource.
func (t *T) reserve(ctx context.Context) (net.IP, net.IP, error) {
	if t.Addr != "" {
		ip := net.ParseIP(t.Addr)
		if ip == nil {
			return nil, nil, fmt.Errorf("addr %q is not an ip address", t.Addr)
		}
		if err := t.alloc().Reserve(ip); err != nil {
			return nil, nil, err
		}
		return ip, nil, nil
	}
	if t.perInstance() {
		// The address of this instance is kept in the reservation store of
		// this node, which holds it from a start to the next until the
		// instance is unprovisioned. Recording it in the configuration, as
		// the address of all the instances is, would have the instances
		// starting at once write it at once, each over the others.
		ip, err := t.alloc().Allocate(ctx)
		return ip, nil, err
	}
	previous, err := t.alloc().Allocated()
	if err != nil {
		return nil, nil, err
	}
	var ip net.IP
	if previous == nil {
		ip, err = t.alloc().Allocate(ctx)
	} else {
		ip, err = t.alloc().Redraw(ctx, previous)
	}
	if err != nil {
		return nil, nil, err
	}
	if err := t.setAddr(ctx, ip.String()); err != nil {
		return nil, nil, fmt.Errorf("record the address %s drawn from network %s: %w", ip, t.Network, err)
	}
	return ip, previous, nil
}

// dropPrevious removes from the interface the address the resource gave up,
// when it is still there: the resource was started with it, and nothing else
// would ever remove it, the resource answering for its new address only.
func (t *T) dropPrevious(previous net.IP) error {
	if previous == nil || previous.Equal(t._ipaddr) {
		return nil
	}
	dev, _ := resip.SplitDevLabel(t.Dev)
	i, err := net.InterfaceByName(dev)
	if err != nil {
		return nil
	}
	addrs, err := i.Addrs()
	if err != nil {
		return err
	}
	if !Addrs(addrs).Has(previous) {
		return nil
	}
	t.Log().Infof("remove %s from %s, the address given up for %s", previous, dev, t._ipaddr)
	return t.addrDel(fmt.Sprintf("%s/%d", previous, t.ipmaskOnes()), dev)
}

// setAddr writes the address drawn from the network in the configuration of
// the object, which the daemon brings to the other nodes, and unsets it when
// empty.
func (t *T) setAddr(ctx context.Context, addr string) error {
	obj, err := object.NewConfigurer(t.Path)
	if err != nil {
		return err
	}
	k := t.addrKey()
	if addr == "" {
		if err := obj.Unset(ctx, k); err != nil {
			return err
		}
		t.Log().Infof("address %s drawn from network %s removed from the configuration", t.Addr, t.Network)
	} else {
		if err := obj.Set(ctx, keyop.T{Key: k, Op: keyop.Set, Value: addr}); err != nil {
			return err
		}
		t.Log().Infof("address %s drawn from network %s recorded in the configuration", addr, t.Network)
	}
	t.Addr = addr
	return nil
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) Abort(ctx context.Context) bool {
	if t.Tags.Has(tagNonRouted) || t.IsActionDisabled() {
		return false // let start fail with an explicit error message
	}
	if t.ipaddr() == nil {
		return false // let start fail with an explicit error message
	}
	if t._ipaddrAge > maxIPAddrAge {
		return false // let start fail with an explicit error message
	}
	if initialStatus := t.statusWithIPAddrCacheTrust(ctx); initialStatus == status.Up {
		return false // let start fail with an explicit error message
	}
	if t.CheckCarrier {
		if carrier, err := t.hasCarrier(); err == nil && carrier == false && !actioncontext.IsForce(ctx) {
			t.Log().Errorf("abort! interface %s no-carrier.", t.Dev)
			return true
		}
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
	timeout := 5 * time.Second
	ip := t.ipaddr().String()
	t.Log().Infof("abort? checking %s availability with ping (%s)", ip, timeout)
	isAlive, err := ping.Ping(ip, timeout)
	if err != nil {
		t.Log().Errorf("abort? ping failed: %s", err)
		return true
	}
	if isAlive {
		t.Log().Errorf("abort! %s is alive", ip)
		return true
	} else {
		t.Log().Tracef("abort? %s is not alive", ip)
		return false
	}
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
	if t.isAllocated() && t.Addr != "" {
		// The address the resource drew, the same on every node.
		t._ipaddr = net.ParseIP(t.Addr)
		return t._ipaddr
	}
	if t.isAllocated() {
		// Reading it never draws one: only a start does.
		ip, err := t.alloc().Allocated()
		if err != nil {
			t.StatusLog().Warn("%s", err)
		}
		t._ipaddr = ip
		return ip
	}
	ip, age, err := getaddr.Lookup(t.Name)
	if getaddr.IsErrManyAddr(err) {
		t.StatusLog().Warn("%s", err)
	}
	t._ipaddr = ip
	t._ipaddrAge = age
	return ip
}

// ipmask returns the mask of the address, and caches it only once the
// address is known: the mask of no address is not the mask of the address a
// start draws, and a status read before the draw would leave it cached.
func (t *T) ipmask() net.IPMask {
	if t._ipmask != nil {
		return t._ipmask
	}
	if t.ipaddr() == nil {
		return nil
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
	if m, err := t.defaultMask(bits); err == nil {
		return m
	}
	return nil
}

// defaultMask returns the mask of the first address of the interface in the
// family of the address, of bits bits: the mask of an ipv4 address of the
// interface is no mask for an ipv6 one.
func (t *T) defaultMask(bits int) (net.IPMask, error) {
	intf, err := net.InterfaceByName(t.Dev)
	if err != nil {
		return nil, err
	}
	addrs, err := intf.Addrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		ip, ipnet, err := net.ParseCIDR(addr.String())
		if err != nil || getIPBits(ip) != bits {
			continue
		}
		return ipnet.Mask, nil
	}
	return nil, fmt.Errorf("no addr to guess mask from")
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

func (t *T) arpAnnounce(dev string) error {
	ip := t.ipaddr()
	if ip.IsLoopback() {
		t.Log().Tracef("skip arp announce on loopback address %s", ip)
		return nil
	}
	if ip.IsLinkLocalUnicast() {
		t.Log().Tracef("skip arp announce on link local unicast address %s", ip)
		return nil
	}
	if i, err := net.InterfaceByName(dev); err == nil && i.Flags&net.FlagLoopback != 0 {
		t.Log().Tracef("skip arp announce on loopback interface %s", t.Dev)
		return nil
	}
	if ip.To4() == nil {
		// The neighbors of the segment learn the address moved, as an ipv4
		// gratuitous arp tells them, rather than keep sending to the node
		// it left until their cache expires.
		t.Log().Infof("send an unsolicited neighbor advertisement to announce %s over %s", ip, dev)
		return t.neighborAdvertise(dev)
	}
	t.Log().Infof("send gratuitous arp to announce %s over %s", t.ipaddr(), dev)
	return t.arpGratuitous(dev)
}

func (t *T) ipmaskOnes() int {
	ones, _ := t.ipmask().Size()
	return ones
}

func (t *T) start(dev, label string) error {
	addr := fmt.Sprintf("%s/%d", t.ipaddr(), t.ipmaskOnes())
	return t.addrAdd(addr, dev, label)
}

func (t *T) stopAddr(ctx context.Context, dev string) error {
	if t.statusOfAddr(ctx, dev) == status.Down {
		t.Log().Infof("%s is already down on %s", t.addrName(), t.Dev)
		return nil
	}
	addr := fmt.Sprintf("%s/%d", t.ipaddr(), t.ipmaskOnes())
	return t.addrDel(addr, t.Dev)
}
