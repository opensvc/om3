// Package networkl2bridge is the l2_bridge network driver: a bridge with no
// address, set up on every node, for the guests and containers of the node
// to share a layer 2 segment that om neither addresses nor routes, as one a
// gateway guest serves.
//
// The bridge is alone on its node unless meshed with the bridges of the
// other nodes, one of two ways the keywords choose:
//
//   - ports: the nodes share a lan or a vlan, and each enslaves its port of
//     it, optionally a vlan interface om creates on it;
//   - vxlan: the nodes share no segment, and each enslaves a vxlan device
//     carrying the frames to the other nodes over their ip network.
//
// The setup never assigns an address to the bridge or its ports, never
// enslaves a port holding one, and never removes a port it did not enslave,
// as the taps of the guests. A bridge it did not create is adopted as it is.
package networkl2bridge

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/netstatus"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	T struct {
		network.T

		// report is what the last setup enslaved and reached, for the
		// status of the setup.
		report netstatus.Status
	}

	// settings are what the setup of the bridge of this node applies, read
	// from the keywords of the network, the values of this node.
	settings struct {
		// name is the name of the network, and nodename the one of
		// this node.
		name     string
		nodename string

		// bridge is the name of the bridge device.
		bridge string

		// mtu is the mtu of the bridge and of the devices om enslaves, 0
		// to leave it, or to compute it for a vxlan.
		mtu int

		// mac is the address of the bridge, nil for a stable one derived
		// from the name of the network on a bridge om creates.
		mac net.HardwareAddr

		// stp, mcastSnooping and ageing are the bridge options, nil to
		// leave them as they are.
		stp           *bool
		mcastSnooping *bool
		ageing        *time.Duration

		// hairpin sets the hairpin mode of the devices om enslaves.
		hairpin bool

		// ports are the devices of this node the bridge enslaves, and
		// vlan the vlan om creates on each to enslave instead, 0 for
		// none.
		ports []string
		vlan  int

		// vni is the vxlan network identifier, 0 for no vxlan, port its
		// udp port, local the address of this node the vxlan sends from
		// and peers the addresses of the other nodes.
		vni       int
		vxlanPort int
		local     net.IP
		peers     []net.IP

		// peerNodes are the names of the nodes of peers, in their order.
		peerNodes []string

		// report, when set, records the ports the setup enslaved and
		// the peers its vxlan sends to, with their failures.
		report *netstatus.Status
	}
)

const (
	// maxDevNameLen is the length a device name is under, IFNAMSIZ.
	maxDevNameLen = 16

	// defaultVXLANPort is the udp port of vxlan the IANA assigned.
	defaultVXLANPort = 4789

	// vxlanOverhead4 and vxlanOverhead6 are the bytes vxlan adds to a
	// frame over ipv4 and ipv6: the mtu of the bridge is the one of the
	// underlay less them.
	vxlanOverhead4 = 50
	vxlanOverhead6 = 70
)

var (
	drvID = driver.NewID(driver.GroupNetwork, "l2_bridge")

	// ipCmd runs ip, a variable for the tests.
	ipCmd = func(args ...string) ([]byte, error) {
		return exec.Command("ip", args...).CombinedOutput()
	}
)

func init() {
	driver.Register(drvID, NewNetworker)
}

func NewNetworker() network.Networker {
	t := New()
	t.SetAllowEmptyNetwork(true)
	var i interface{} = t
	return i.(network.Networker)
}

func New() *T {
	return &T{}
}

// brName returns the name of the bridge: the dev keyword, else obr_ and the
// name of the network.
func (t *T) brName() string {
	if s := t.GetString("dev"); s != "" {
		return s
	}
	return "obr_" + t.Name()
}

// BackendDevName returns the bridge, which the firewall accepts the frames
// forwarded through, and whose link coming up sets the network up again.
func (t *T) BackendDevName() string {
	return t.brName()
}

// IsLayer2 says the network is a layer 2 segment om neither addresses nor
// routes: an ip.netns joining it is plugged into the bridge, and holds the
// address its configuration names, or none.
func (t *T) IsLayer2() bool {
	return true
}

// PortDevNames returns the devices of this node the bridge enslaves, whose
// link coming up sets the network up again: a port missing at the setup is
// enslaved once it appears.
func (t *T) PortDevNames() []string {
	return t.GetStrings("ports")
}

// statePath returns the file of the state of the network on the node.
func (t *T) statePath() string {
	return statePath(t.Name())
}

func statePath(name string) string {
	return filepath.Join(rawconfig.NodeVarDir(), "network", name+".l2_bridge.json")
}

// vxlanName returns the name of the vxlan device of the network: vx_ and its
// name, or a hash of it when too long for a device name.
func vxlanName(name string) string {
	s := "vx_" + name
	if len(s) < maxDevNameLen {
		return s
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return fmt.Sprintf("vx_%08x", h.Sum32())
}

// vlanName returns the name of the vlan interface of port.
func vlanName(port string, vlan int) string {
	return port + "." + strconv.Itoa(vlan)
}

// stableMAC returns a locally administered unicast address derived from the
// name of the network and of the node, the same at every setup: a bridge
// takes the lowest address of its ports otherwise, which changes as the taps
// of the guests come and go, and with it the address its neighbours know it
// by. It differs from node to node, the bridges of a mesh being on one
// segment, where the same address on two of them collides, as in their
// spanning tree bridge identifiers.
func stableMAC(name, nodename string) net.HardwareAddr {
	sum := sha256.Sum256([]byte(name + "\x00" + nodename))
	mac := make(net.HardwareAddr, 6)
	copy(mac, sum[:6])
	// Locally administered, unicast.
	mac[0] = mac[0]&0xfc | 0x02
	return mac
}

// optionalBool parses s as a boolean, nil when empty.
func optionalBool(s string) (*bool, error) {
	if s == "" {
		return nil, nil
	}
	v, err := converters.Bool.Convert(s)
	if err != nil {
		return nil, err
	}
	b := v.(bool)
	return &b, nil
}

// settings returns what the setup of this node applies, from the keywords.
func (t *T) settings() (settings, error) {
	var (
		s   settings
		err error
	)
	s.name = t.Name()
	s.nodename = hostname.Hostname()
	s.bridge = t.brName()
	if len(s.bridge) >= maxDevNameLen {
		return s, fmt.Errorf("the bridge name %s is longer than %d characters: set dev", s.bridge, maxDevNameLen-1)
	}
	if v := t.GetString("mtu"); v != "" {
		if s.mtu, err = strconv.Atoi(v); err != nil {
			return s, fmt.Errorf("mtu: %w", err)
		}
	}
	if v := t.GetString("mac"); v != "" {
		if s.mac, err = net.ParseMAC(v); err != nil {
			return s, fmt.Errorf("mac: %w", err)
		}
	}
	if s.stp, err = optionalBool(t.GetString("stp")); err != nil {
		return s, fmt.Errorf("stp: %w", err)
	}
	if s.mcastSnooping, err = optionalBool(t.GetString("mcast_snooping")); err != nil {
		return s, fmt.Errorf("mcast_snooping: %w", err)
	}
	if v := t.GetString("ageing_time"); v != "" {
		d, err := converters.Duration.Convert(v)
		if err != nil {
			return s, fmt.Errorf("ageing_time: %w", err)
		}
		s.ageing = d.(*time.Duration)
	}
	s.hairpin = t.GetBool("hairpin")
	s.ports = t.GetStrings("ports")
	if v := t.GetString("vlan"); v != "" {
		if s.vlan, err = strconv.Atoi(v); err != nil {
			return s, fmt.Errorf("vlan: %w", err)
		}
	}
	if v := t.GetString("vxlan_vni"); v != "" {
		if s.vni, err = strconv.Atoi(v); err != nil {
			return s, fmt.Errorf("vxlan_vni: %w", err)
		}
	}
	if s.vni == 0 {
		return s, nil
	}
	s.vxlanPort = defaultVXLANPort
	if v := t.GetString("vxlan_port"); v != "" {
		if s.vxlanPort, err = strconv.Atoi(v); err != nil {
			return s, fmt.Errorf("vxlan_port: %w", err)
		}
	}
	if s.local, err = t.vxlanAddr(hostname.Hostname(), ""); err != nil {
		return s, fmt.Errorf("vxlan address of this node: %w", err)
	}
	nodes, err := t.Nodes()
	if err != nil {
		return s, err
	}
	af := "ip4"
	if s.local.To4() == nil {
		af = "ip6"
	}
	for _, nodename := range nodes {
		if nodename == hostname.Hostname() {
			continue
		}
		ip, err := t.vxlanAddr(nodename, af)
		if err != nil {
			return s, fmt.Errorf("vxlan address of node %s: %w", nodename, err)
		}
		s.peers = append(s.peers, ip)
		s.peerNodes = append(s.peerNodes, nodename)
	}
	return s, nil
}

// vxlanAddr returns the address of nodename the vxlan sends from and to: the
// vxlan_addr keyword of the node, an address or a name to resolve, else the
// address the name of the node resolves to, of the family af when set.
func (t *T) vxlanAddr(nodename, af string) (net.IP, error) {
	var addr string
	if nodename == hostname.Hostname() {
		addr = t.GetString("vxlan_addr")
	} else {
		addr = t.GetStringAs("vxlan_addr", nodename)
	}
	if addr == "" {
		addr = nodename
	}
	if ip := net.ParseIP(addr); ip != nil {
		return ip, nil
	}
	if af == "" {
		af = "ip"
	}
	return network.GetNodeAddr(addr, af)
}

// VXLAN returns the udp port of the vxlan of the network, the address of
// this node it receives on, and the addresses of the nodes it receives from,
// for the firewall to drop the vxlan of any
// other host: vxlan is neither authenticated nor encrypted. A port of 0 is
// no vxlan.
//
// A vxlan the configuration asks for but that it cannot describe, an
// address of a node not resolving for example, is the port with no address:
// the firewall drops all of it. Its setup fails before the device, which
// the previous setup made and is left up, and the vxlan of any host would
// reach the bridge else.
func (t *T) VXLAN() (int, net.IP, []net.IP) {
	s, err := t.settings()
	if err == nil {
		if s.vni == 0 {
			return 0, nil, nil
		}
		return s.vxlanPort, s.local, s.peers
	}
	if t.GetString("vxlan_vni") == "" {
		return 0, nil, nil
	}
	port := defaultVXLANPort
	if v := t.GetString("vxlan_port"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			port = i
		}
	}
	t.Log().Warnf("vxlan dropped by the firewall: %s", err)
	return port, nil, nil
}

func (t *T) Setup() error {
	s, err := t.settings()
	if err != nil {
		return err
	}
	t.report = netstatus.Status{}
	s.report = &t.report
	p := t.statePath()
	st, err := loadState(p)
	if err != nil {
		return fmt.Errorf("load the state %s: %w", p, err)
	}
	err = apply(t.Log(), s, &st)
	if serr := st.save(p); serr != nil && err == nil {
		err = fmt.Errorf("save the state %s: %w", p, serr)
	}
	return err
}

// Report adds to the status of the setup the ports it enslaved, and the
// nodes its vxlan sends to, with their failures.
func (t *T) Report(st *netstatus.Status) {
	st.Ports = t.report.Ports
	st.Peers = t.report.Peers
}

// Teardown undoes the setup, the configuration of the network removed: the
// ports om enslaved are released, the vlan and vxlan devices it created
// deleted, and the bridge it created deleted once nothing else is enslaved
// to it. A bridge it adopted is left.
func (t *T) Teardown() error {
	p := t.statePath()
	st, err := loadState(p)
	if err != nil {
		return fmt.Errorf("load the state %s: %w", p, err)
	}
	bridge := t.brName()
	if st.Bridge != "" {
		bridge = st.Bridge
	}
	if err := teardown(t.Log(), bridge, &st); err != nil {
		if serr := st.save(p); serr != nil {
			t.Log().Warnf("save the state %s: %s", p, serr)
		}
		return err
	}
	return removeState(p)
}

// bridgeTypeArgs returns the arguments of the ip command setting the bridge
// options, none when no option is set.
func bridgeTypeArgs(s settings) []string {
	var l []string
	if s.stp != nil {
		l = append(l, "stp_state", boolArg(*s.stp))
	}
	if s.mcastSnooping != nil {
		l = append(l, "mcast_snooping", boolArg(*s.mcastSnooping))
	}
	if s.ageing != nil {
		// In hundredths of a second.
		l = append(l, "ageing_time", strconv.FormatInt(int64(*s.ageing/(10*time.Millisecond)), 10))
	}
	if len(l) == 0 {
		return nil
	}
	return append([]string{"link", "set", "dev", s.bridge, "type", "bridge"}, l...)
}

func boolArg(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// ensureBridge returns the bridge of s, created if absent and recorded as
// created by om in st. A device of the name that is no bridge is an error.
func ensureBridge(log *plog.Logger, s settings, st *state) (netlink.Link, error) {
	link, err := netlink.LinkByName(s.bridge)
	if err == nil {
		if link.Type() != "bridge" {
			return nil, fmt.Errorf("%s is a %s device, not a bridge", s.bridge, link.Type())
		}
		return link, nil
	}
	if _, ok := err.(netlink.LinkNotFoundError); !ok {
		return nil, err
	}
	la := netlink.NewLinkAttrs()
	la.Name = s.bridge
	if err := netlink.LinkAdd(&netlink.Bridge{LinkAttrs: la}); err != nil {
		return nil, fmt.Errorf("create the bridge %s: %w", s.bridge, err)
	}
	log.Infof("created the bridge %s", s.bridge)
	st.CreatedBridge = true
	return netlink.LinkByName(s.bridge)
}

// apply sets up the bridge of s, its options and its mesh, and records in
// st what it made.
func apply(log *plog.Logger, s settings, st *state) error {
	if len(s.ports) > 0 && s.vni > 0 {
		// Both would join the segment twice to the other nodes: a loop.
		return fmt.Errorf("ports and vxlan_vni are two ways to mesh the bridges of the nodes, set one")
	}
	if st.Bridge != "" && st.Bridge != s.bridge {
		// What the state records is of the former bridge: kept, the
		// bridge adopted would be taken for one om created, its ipv6
		// disabled and itself deleted at the teardown.
		log.Infof("the bridge is now %s: undo the setup of %s", s.bridge, st.Bridge)
		if err := teardown(log, st.Bridge, st); err != nil {
			return fmt.Errorf("undo the setup of the former bridge %s: %w", st.Bridge, err)
		}
	}
	st.Bridge = s.bridge
	br, err := ensureBridge(log, s, st)
	if err != nil {
		return err
	}
	if st.CreatedBridge {
		// No address on a segment om does not address: the node is
		// not to be reachable from the guests on it by an ipv6 link
		// local one.
		if err := disableIPv6(s.bridge); err != nil {
			return err
		}
	}
	mac := s.mac
	if mac == nil && st.CreatedBridge {
		mac = stableMAC(s.name, s.nodename)
	}
	if mac != nil && br.Attrs().HardwareAddr.String() != mac.String() {
		if err := netlink.LinkSetHardwareAddr(br, mac); err != nil {
			return fmt.Errorf("set the address of %s to %s: %w", s.bridge, mac, err)
		}
		log.Infof("set the address of %s to %s", s.bridge, mac)
	}
	if args := bridgeTypeArgs(s); args != nil {
		if b, err := ipCmd(args...); err != nil {
			return fmt.Errorf("ip %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(b)))
		}
	}
	mtu := s.mtu
	if s.vni > 0 && mtu == 0 {
		if mtu, err = vxlanMTU(s.local); err != nil {
			return err
		}
	}
	if mtu > 0 && br.Attrs().MTU != mtu {
		if err := netlink.LinkSetMTU(br, mtu); err != nil {
			return fmt.Errorf("set the mtu of %s to %d: %w", s.bridge, mtu, err)
		}
		log.Infof("set the mtu of %s to %d", s.bridge, mtu)
	}
	// The bridge is up before its mesh, so the guests of the node share
	// it even while a port is missing or the vxlan is refused.
	if err := netlink.LinkSetUp(br); err != nil {
		return fmt.Errorf("set %s up: %w", s.bridge, err)
	}
	return errors.Join(
		applyPorts(log, s, mtu, br, st),
		applyVXLAN(log, s, mtu, br, st),
	)
}

// disableIPv6 disables ipv6 on the device dev, which removes its addresses,
// the link local one included. A kernel with no ipv6 has none to remove.
func disableIPv6(dev string) error {
	p := filepath.Join("/proc/sys/net/ipv6/conf", dev, "disable_ipv6")
	if b, err := os.ReadFile(p); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err == nil && strings.TrimSpace(string(b)) == "1" {
		return nil
	}
	if err := os.WriteFile(p, []byte("1"), 0o644); err != nil {
		return fmt.Errorf("disable ipv6 on %s: %w", dev, err)
	}
	return nil
}

// teardown undoes what st records of the setup of the bridge named bridge.
func teardown(log *plog.Logger, bridge string, st *state) error {
	for _, dev := range st.Ports {
		releasePort(log, dev, bridge)
	}
	st.Ports = nil
	for _, dev := range st.VLANs {
		if err := delLink(log, dev); err != nil {
			return err
		}
	}
	st.VLANs = nil
	if st.VXLAN != "" {
		if err := delLink(log, st.VXLAN); err != nil {
			return err
		}
		st.VXLAN = ""
	}
	if !st.CreatedBridge {
		log.Infof("leave the bridge %s om did not create", bridge)
		st.Bridge = ""
		return nil
	}
	br, err := netlink.LinkByName(bridge)
	if _, ok := err.(netlink.LinkNotFoundError); ok {
		st.CreatedBridge = false
		st.Bridge = ""
		return nil
	} else if err != nil {
		return err
	}
	if held := enslaved(br); len(held) > 0 {
		return fmt.Errorf("the bridge %s still holds %s, which om did not enslave: it is deleted at a network setup once they are gone", bridge, strings.Join(held, " "))
	}
	if err := delLink(log, bridge); err != nil {
		return err
	}
	st.CreatedBridge = false
	st.Bridge = ""
	return nil
}

// enslaved returns the names of the devices enslaved to br.
func enslaved(br netlink.Link) []string {
	links, err := netlink.LinkList()
	if err != nil {
		return nil
	}
	var l []string
	for _, link := range links {
		if link.Attrs().MasterIndex == br.Attrs().Index {
			l = append(l, link.Attrs().Name)
		}
	}
	return l
}

func delLink(log *plog.Logger, name string) error {
	link, err := netlink.LinkByName(name)
	if _, ok := err.(netlink.LinkNotFoundError); ok {
		return nil
	} else if err != nil {
		return err
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("delete %s: %w", name, err)
	}
	log.Infof("deleted %s", name)
	return nil
}
