package rescontainerkvm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/drivers/resip"
	"github.com/opensvc/om3/v3/util/command"
	ulock "github.com/opensvc/om3/v3/util/lock"
)

const (
	// filterTable holds a chain per network interface of a guest.
	filterTable = "osvc_guests"

	guestNetLock = "/var/lock/opensvc.guestnet.lock"

	// routerLinkLocal is the address every om bridge has, which the route to
	// the whole of an ipv6 network goes through: DHCPv6 can not say it.
	routerLinkLocal = "fe80::1"
)

// guestNetDir holds what dnsmasq reads, as nobody: under the run directory of
// om, only root would.
var guestNetDir = "/run/opensvc-guests"

type (
	// guestNIC is a network interface of a guest, on an om network.
	guestNIC struct {
		network string

		// owner is the resource the interface is of.
		owner string

		// dev and mac are the same on every node, as the definition of the
		// domain the nodes share says them.
		dev string
		mac string

		// master is the bridge the tap is a port of, an openvswitch one when
		// ovs is set, in the vlan of tag.
		master   string
		ovs      bool
		vlanTag  string
		vlanMode string

		ip net.IP

		// subnet is what the node draws from, and whole the network.
		subnet *net.IPNet
		whole  *net.IPNet

		gateway     net.IP
		prefix      int
		nameservers []string

		// static says the address is the cluster's, on a segment no dnsmasq
		// of om may answer on: the guest is told it at its first boot, by
		// cloud-init, and keeps it.
		static bool
	}
)

// nicNames returns the name and the mac of the network interface of rank i of
// a resource. A digest rather than a mere hash, as the name of an object is
// what its user chooses, though they fit too few bits to tell objects apart
// for sure: claimNIC refuses what they collide with.
func nicNames(resourceKey string, i int) (string, string) {
	dev := sha256.Sum256([]byte(resourceKey))
	mac := sha256.Sum256([]byte(fmt.Sprintf("%s!%d", resourceKey, i)))
	return fmt.Sprintf("kv%sn%d", hex.EncodeToString(dev[:5]), i),
		fmt.Sprintf("52:54:%02x:%02x:%02x:%02x", mac[0], mac[1], mac[2], mac[3])
}

// nicRecord is what claimNIC records of an interface of this node.
func nicRecord(n guestNIC) string {
	return n.owner + "\n" + n.mac + "\n"
}

// claimNIC records the interface as the guest's, refusing what is another
// guest's: unplugging an interface of the name would cut that guest, and
// sharing its mac would have the bridge send its traffic here.
func claimNIC(n guestNIC) error {
	dir := filepath.Join(guestNetDir, "nics")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	l, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range l {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		owner, mac, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
		switch {
		case owner == n.owner:
		case e.Name() == n.dev:
			return fmt.Errorf("interface %s is held by %s", n.dev, owner)
		case mac == n.mac:
			return fmt.Errorf("mac %s is held by %s", n.mac, owner)
		}
	}
	return os.WriteFile(filepath.Join(dir, n.dev), []byte(nicRecord(n)), 0o600)
}

// ownsNIC says the interface is the guest's. One no record says anything of
// is: only a guest of this resource names it, the record being of the guests
// started with it.
func ownsNIC(n guestNIC) bool {
	b, err := os.ReadFile(filepath.Join(guestNetDir, "nics", n.dev))
	if err != nil {
		return true
	}
	owner, _, _ := strings.Cut(string(b), "\n")
	return owner == n.owner
}

// releaseNIC forgets the interface, once unplugged.
func releaseNIC(n guestNIC) {
	if ownsNIC(n) {
		_ = os.Remove(filepath.Join(guestNetDir, "nics", n.dev))
	}
}

// isIP6 says the interface carries ipv6, a network being of one family.
func (t guestNIC) isIP6() bool {
	return t.ip.To4() == nil
}

// linkLocal returns the eui-64 link-local address of the mac, the only one
// the filter lets a guest use: any other could be the router's.
func linkLocal(mac string) (net.IP, error) {
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return nil, err
	}
	if len(hw) != 6 {
		return nil, fmt.Errorf("%s is not an ethernet address", mac)
	}
	ip := make(net.IP, net.IPv6len)
	ip[0], ip[1] = 0xfe, 0x80
	copy(ip[8:], []byte{hw[0] ^ 2, hw[1], hw[2], 0xff, 0xfe, hw[3], hw[4], hw[5]})
	return ip, nil
}

func ip6Hex(ip net.IP) string {
	return "0x" + hex.EncodeToString(ip.To16())
}

// filterRules returns what nft loads to keep a guest to its own addresses.
// Everything the guest sends is dropped but what names its mac and its
// addresses: it can not take the address of another, whatever it configures,
// nor answer as a router or a DHCP server.
//
// The ingress hook of a device filters what enters the host through it, which
// is what the guest writes to a tap, whichever bridge the tap is a port of.
func filterRules(n guestNIC) (string, error) {
	ll, err := linkLocal(n.mac)
	if err != nil {
		return "", err
	}
	l := []string{
		fmt.Sprintf("ether saddr != %s drop", n.mac),
		"ether type ip6 icmpv6 type { nd-router-advert, nd-redirect } drop",
		"ether type ip6 udp sport 547 drop",
		"ether type ip udp sport 67 drop",
	}
	own6 := []string{ll.String(), "::"}
	targets := []string{ip6Hex(ll)}
	if n.isIP6() {
		own6 = append(own6, n.ip.String())
		targets = append(targets, ip6Hex(n.ip))
	} else {
		l = append(l,
			fmt.Sprintf("ether type arp arp saddr ether %s arp saddr ip { %s, 0.0.0.0 } accept", n.mac, n.ip),
			"ether type ip ip saddr 0.0.0.0 ip daddr 255.255.255.255 udp sport 68 udp dport 67 accept",
			fmt.Sprintf("ether type ip ip saddr %s accept", n.ip),
		)
	}
	l = append(l,
		fmt.Sprintf("ether type ip6 icmpv6 type nd-neighbor-advert @th,64,128 { %s } accept", strings.Join(targets, ", ")),
		"ether type ip6 icmpv6 type nd-neighbor-advert drop",
		fmt.Sprintf("ether type ip6 ip6 saddr { %s } accept", strings.Join(own6, ", ")),
	)
	return fmt.Sprintf("table netdev %s {\n  chain %s {\n    type filter hook ingress device %q priority -500; policy drop;\n    %s\n  }\n}\n",
		filterTable, n.dev, n.dev, strings.Join(l, "\n    ")), nil
}

// networkConfig returns the cloud-init network configuration of the guest.
//
// It says what a guest would not do by itself: use all its interfaces, keep
// the eui-64 link-local address the filter expects, and, on a lan, the address
// no dhcp hands out. An interface of an ipv4 network has no ipv6 at all.
func networkConfig(nics []guestNIC) string {
	var b strings.Builder
	b.WriteString("version: 2\nethernets:\n")
	for i, n := range nics {
		fmt.Fprintf(&b, "  nic%d:\n    match:\n      macaddress: %q\n", i, n.mac)
		dhcp := !n.static
		fmt.Fprintf(&b, "    dhcp4: %t\n    dhcp6: %t\n", dhcp && !n.isIP6(), dhcp && n.isIP6())
		if n.isIP6() {
			b.WriteString("    ipv6-address-generation: eui64\n")
			fmt.Fprintf(&b, "    accept-ra: %t\n", dhcp)
		} else {
			b.WriteString("    link-local: []\n")
		}
		if n.static {
			fmt.Fprintf(&b, "    addresses:\n      - %s/%d\n", n.ip, n.prefix)
			if n.gateway != nil {
				fmt.Fprintf(&b, "    routes:\n      - to: default\n        via: %s\n", n.gateway)
			}
		} else if n.isIP6() && n.whole != nil {
			fmt.Fprintf(&b, "    routes:\n      - to: %s\n        via: %q\n        on-link: true\n", n.whole, routerLinkLocal)
		}
		if n.static && len(n.nameservers) > 0 {
			fmt.Fprintf(&b, "    nameservers:\n      addresses: [%s]\n", strings.Join(n.nameservers, ", "))
		}
	}
	return b.String()
}

// dnsmasqArgs returns the options of the dnsmasq of a bridge, which answers
// the guests it has a host entry for, and no other. It serves no dns.
func dnsmasqArgs(dir string, n guestNIC) []string {
	maskOnes, _ := n.subnet.Mask.Size()
	l := []string{
		"--conf-file=/dev/null",
		"--port=0",
		"--bind-interfaces",
		"--interface=" + n.master,
		"--except-interface=lo",
		"--leasefile-ro",
		"--dhcp-hostsdir=" + filepath.Join(dir, "hosts"),
		"--pid-file=" + filepath.Join(dir, "pid"),
	}
	if n.isIP6() {
		l = append(l,
			fmt.Sprintf("--dhcp-range=%s,static,%d,infinite", n.subnet.IP, maskOnes),
			"--enable-ra",
		)
		if len(n.nameservers) > 0 {
			l = append(l, "--dhcp-option=option6:dns-server,"+wrapIP6(n.nameservers))
		}
		return l
	}
	l = append(l,
		fmt.Sprintf("--dhcp-range=%s,static,%s,infinite", n.subnet.IP, net.IP(n.subnet.Mask)),
		fmt.Sprintf("--dhcp-option=3,%s", n.gateway),
		fmt.Sprintf("--dhcp-option=121,%s,%s,0.0.0.0/0,%s", n.whole, n.gateway, n.gateway),
	)
	if len(n.nameservers) > 0 {
		l = append(l, "--dhcp-option=6,"+strings.Join(n.nameservers, ","))
	}
	return l
}

func wrapIP6(l []string) string {
	wrapped := make([]string, len(l))
	for i, s := range l {
		wrapped[i] = "[" + s + "]"
	}
	return strings.Join(wrapped, ",")
}

// dhcpHost returns the entry of the guest in the hosts of dnsmasq.
func dhcpHost(n guestNIC) string {
	if n.isIP6() {
		return fmt.Sprintf("%s,[%s]\n", n.mac, n.ip)
	}
	return fmt.Sprintf("%s,%s\n", n.mac, n.ip)
}

func (t *T) allocationOf(name string) *resip.Allocation {
	return &resip.Allocation{Network: name, Path: t.Path, RID: t.RID(), Log: t.Log()}
}

// guestNICs returns the network interfaces of the guest, with the addresses
// they hold: drawing the ones they miss when draw is set, and leaving the
// others without an ip otherwise, a status taking no address.
func (t *T) guestNICs(ctx context.Context, draw bool) ([]guestNIC, error) {
	nics := make([]guestNIC, 0, len(t.Network))
	key := ipam.Key(t.Path, t.RID())
	for i, name := range t.Network {
		n := guestNIC{network: name, owner: key}
		n.dev, n.mac = nicNames(key, i)
		alloc := t.allocationOf(name)
		nw, err := alloc.Resolve()
		if err != nil {
			return nil, err
		}
		if nw == nil {
			return nil, fmt.Errorf("network %s: not a network name", name)
		}
		if err := n.configure(nw); err != nil {
			return nil, err
		}
		a, err := alloc.Allocator()
		if err != nil {
			return nil, err
		}
		if a == nil {
			return nil, fmt.Errorf("network %s: om allocates no address in it", name)
		}
		n.subnet = a.Range
		if n.gateway == nil && !n.static {
			n.gateway = ipam.Gateway(a.Range)
		}
		if n.prefix == 0 {
			n.prefix, _ = a.Range.Mask.Size()
		}
		if draw {
			n.ip, err = alloc.Allocate(ctx)
		} else {
			n.ip, err = alloc.Allocated()
		}
		if err != nil {
			return nil, err
		}
		nics = append(nics, n)
	}
	return nics, nil
}

// configure fills from the network what the interface plugs into.
func (t *guestNIC) configure(nw network.Networker) error {
	var err error
	if t.whole, err = nw.IPNet(); err != nil {
		return err
	}
	if i, ok := nw.(network.Nameserverer); ok {
		t.nameservers = i.Nameservers()
	}
	i, ok := nw.(network.HostDever)
	if !ok {
		d, ok := nw.(interface{ BackendDevName() string })
		if !ok || d.BackendDevName() == "" {
			return fmt.Errorf("network %s: a guest plugs into a bridge, and this %s network has none", t.network, nw.Type())
		}
		t.master = d.BackendDevName()
		return nil
	}
	t.static = true
	if d, ok := nw.(network.DevNamer); ok {
		t.master = d.NamedDev()
	}
	if t.master == "" {
		if t.master, err = i.HostDev(); err != nil {
			return fmt.Errorf("network %s: %w", t.network, err)
		}
	}
	switch {
	case isLinuxBridge(t.master):
	case isOVSBridge(t.master):
		t.ovs = true
		if v, ok := nw.(network.VLANer); ok {
			t.vlanTag, t.vlanMode = v.VLAN()
		}
	default:
		return fmt.Errorf("network %s: a guest plugs into a bridge, and %s is neither a linux nor an openvswitch one", t.network, t.master)
	}
	if m, ok := nw.(network.Netmasker); ok {
		if t.prefix, err = m.Netmask(); err != nil {
			return err
		}
	}
	if g, ok := nw.(network.Gatewayer); ok {
		if t.gateway, err = g.Gateway(); err != nil {
			return err
		}
	}
	return nil
}

func isLinuxBridge(dev string) bool {
	_, err := os.Stat(filepath.Join("/sys/class/net", dev, "bridge"))
	return err == nil
}

func isOVSBridge(dev string) bool {
	return exec.Command("ovs-vsctl", "br-exists", dev).Run() == nil
}

func (t *T) runNet(ctx context.Context, name string, args ...string) error {
	return command.New(
		command.WithContext(ctx),
		command.WithName(name),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func lockGuestNet() (func(), error) {
	lock := flock.New(guestNetLock, "", fcntllock.New)
	if err := lock.Lock(20*time.Second, ""); err != nil {
		return nil, ulock.Explain(lock, 20*time.Second, err)
	}
	return func() { _ = lock.UnLock() }, nil
}

// networkUp makes what the interfaces of the guest need before libvirt opens
// them, which it does not create, and returns them.
//
// A filter is bound to a device: the tap has to exist for the rules to load,
// and it has to be filtered before it is plugged and the guest runs.
func (t *T) networkUp(ctx context.Context) ([]guestNIC, error) {
	if len(t.Network) == 0 {
		return nil, nil
	}
	if t.NetworkFilter {
		if _, err := exec.LookPath("nft"); err != nil {
			return nil, fmt.Errorf("network: a guest is kept to its addresses by nft, which this node has not: %w", err)
		}
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.networkDown(ctx)
	})
	nics, err := t.guestNICs(ctx, true)
	if err != nil {
		return nil, err
	}
	unlock, err := lockGuestNet()
	if err != nil {
		return nil, err
	}
	defer unlock()
	for _, n := range nics {
		if err := claimNIC(n); err != nil {
			return nil, fmt.Errorf("network %s: %w", n.network, err)
		}
		t.unplug(ctx, n)
		if err := t.plug(ctx, n); err != nil {
			return nil, fmt.Errorf("network %s: %w", n.network, err)
		}
		if n.static {
			continue
		}
		if err := t.serveDHCP(ctx, n); err != nil {
			return nil, fmt.Errorf("network %s: %w", n.network, err)
		}
	}
	return nics, nil
}

func (t *T) plug(ctx context.Context, n guestNIC) error {
	if err := t.runNet(ctx, "ip", "tuntap", "add", "dev", n.dev, "mode", "tap"); err != nil {
		return err
	}
	if t.NetworkFilter {
		rules, err := filterRules(n)
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, "nft", "-f", "-")
		cmd.Stdin = strings.NewReader(rules)
		if b, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("load the filter of %s: %w: %s", n.dev, err, b)
		}
	}
	if master, err := net.InterfaceByName(n.master); err != nil {
		return err
	} else if err := t.runNet(ctx, "ip", "link", "set", n.dev, "mtu", strconv.Itoa(master.MTU)); err != nil {
		return err
	}
	if n.ovs {
		args := []string{"--may-exist", "add-port", n.master, n.dev}
		if n.vlanMode != "" {
			args = append(args, "vlan_mode="+n.vlanMode)
		}
		if n.vlanTag != "" {
			args = append(args, "tag="+n.vlanTag)
		}
		if err := t.runNet(ctx, "ovs-vsctl", args...); err != nil {
			return err
		}
	} else if err := t.runNet(ctx, "ip", "link", "set", n.dev, "master", n.master); err != nil {
		return err
	}
	return t.runNet(ctx, "ip", "link", "set", n.dev, "up")
}

// unplug removes what plug made, what is not there being nothing to remove.
func (t *T) unplug(ctx context.Context, n guestNIC) {
	if !ownsNIC(n) {
		return
	}
	if _, err := net.InterfaceByName(n.dev); err != nil {
		return
	}
	if t.NetworkFilter {
		_ = exec.CommandContext(ctx, "nft", "delete", "chain", "netdev", filterTable, n.dev).Run()
	}
	if n.ovs {
		_ = t.runNet(ctx, "ovs-vsctl", "--if-exists", "del-port", n.master, n.dev)
	}
	_ = t.runNet(ctx, "ip", "link", "del", n.dev)
}

func bridgeDir(master string) string {
	return filepath.Join(guestNetDir, master)
}

// serveDHCP adds the guest to the hosts of the dnsmasq of its bridge, started
// when the first guest of the bridge is, and reloaded otherwise. A dnsmasq
// survives the daemon, and answers the guests whatever becomes of the om
// command that started it.
func (t *T) serveDHCP(ctx context.Context, n guestNIC) error {
	dir := bridgeDir(n.master)
	hosts := filepath.Join(dir, "hosts")
	if err := os.MkdirAll(hosts, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(guestNetDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(hosts, n.dev), []byte(dhcpHost(n)), 0o644); err != nil {
		return err
	}
	args := dnsmasqArgs(dir, n)
	stored := filepath.Join(dir, "args")
	want := strings.Join(args, "\n")
	pid := dnsmasqPID(dir)
	if pid != 0 {
		if b, err := os.ReadFile(stored); err == nil && string(b) == want {
			return syscall.Kill(pid, syscall.SIGHUP)
		}
		// The network changed: its range or its gateway are in the options.
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return err
		}
	}
	if err := os.WriteFile(stored, []byte(want), 0o644); err != nil {
		return err
	}
	return t.runNet(ctx, "dnsmasq", args...)
}

// dnsmasqPID returns the pid of the dnsmasq of the bridge, 0 when none runs.
func dnsmasqPID(dir string) int {
	b, err := os.ReadFile(filepath.Join(dir, "pid"))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil || strings.TrimSpace(string(comm)) != "dnsmasq" {
		return 0
	}
	return pid
}

// stopDHCP removes the guest from the hosts of the dnsmasq of its bridge,
// which stops with the last guest.
func (t *T) stopDHCP(n guestNIC) error {
	dir := bridgeDir(n.master)
	hosts := filepath.Join(dir, "hosts")
	if err := os.Remove(filepath.Join(hosts, n.dev)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	pid := dnsmasqPID(dir)
	if pid == 0 {
		return nil
	}
	l, err := os.ReadDir(hosts)
	if err != nil {
		return err
	}
	if len(l) == 0 {
		t.Log().Infof("stop the dnsmasq of bridge %s: no guest left", n.master)
		return syscall.Kill(pid, syscall.SIGTERM)
	}
	return syscall.Kill(pid, syscall.SIGHUP)
}

// networkDown removes what the interfaces of the guest needed to start. The
// address of a per-node network is released, the next start drawing it where
// it runs then. The one of a lan is the guest's, which was told it once.
func (t *T) networkDown(ctx context.Context) error {
	if len(t.Network) == 0 {
		return nil
	}
	nics, err := t.guestNICs(ctx, false)
	if err != nil {
		return err
	}
	unlock, err := lockGuestNet()
	if err != nil {
		return err
	}
	defer unlock()
	var errs error
	for _, n := range nics {
		t.unplug(ctx, n)
		releaseNIC(n)
		if n.static {
			continue
		}
		errs = errors.Join(errs, t.stopDHCP(n), t.allocationOf(n.network).Free())
	}
	return errs
}

// statusNetwork warns of what the start of a running guest set up and that is
// gone: the guest runs on, its neighbours no longer protected from it, and
// nothing else would tell.
func (t *T) statusNetwork(ctx context.Context) {
	if len(t.Network) == 0 {
		return
	}
	nics, err := t.guestNICs(ctx, false)
	if err != nil {
		t.StatusLog().Warn("network: %s", err)
		return
	}
	for _, n := range nics {
		if t.NetworkFilter && !filterLoaded(ctx, n.dev) {
			t.StatusLog().Warn("interface %s on network %s is not filtered: the guest can take any address", n.dev, n.network)
		}
		if !n.static && dnsmasqPID(bridgeDir(n.master)) == 0 {
			t.StatusLog().Warn("no dnsmasq on bridge %s: the guest gets no address at its next boot", n.master)
		}
	}
}

func filterLoaded(ctx context.Context, dev string) bool {
	return exec.CommandContext(ctx, "nft", "list", "chain", "netdev", filterTable, dev).Run() == nil
}

// releaseAddrs gives up the addresses the guest holds on this node.
func (t *T) releaseAddrs() error {
	var errs error
	for _, name := range t.Network {
		errs = errors.Join(errs, t.allocationOf(name).Free())
	}
	return errs
}

// guestAddrs returns the addresses the guest holds on this node, reading
// reservations only: a status takes no address.
func (t *T) guestAddrs() []string {
	l := make([]string, 0, len(t.Network))
	for _, name := range t.Network {
		ip, err := t.allocationOf(name).Allocated()
		if err != nil {
			t.Log().Debugf("network %s: %s", name, err)
			continue
		}
		if ip != nil {
			l = append(l, ip.String())
		}
	}
	return l
}

// specNetworkError says what is wrong with the network keyword.
func (t *T) specNetworkError() error {
	seen := make([]string, 0, len(t.Network))
	for _, name := range t.Network {
		if slices.Contains(seen, name) {
			return fmt.Errorf("network %s is named twice: a guest holds one address per network", name)
		}
		seen = append(seen, name)
	}
	return nil
}
