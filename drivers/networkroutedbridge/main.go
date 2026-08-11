//go:build linux

package networkroutedbridge

import (
	"fmt"
	"math/big"
	"math/bits"
	"net"
	"strings"

	"github.com/vishvananda/netlink"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	T struct {
		network.T
		subnetMap map[string]string
	}
)

var (
	drvID = driver.NewID(driver.GroupNetwork, "routed_bridge")
)

func init() {
	driver.Register(drvID, NewNetworker)
}

func NewNetworker() network.Networker {
	t := New()
	var i interface{} = t
	return i.(network.Networker)
}

func New() *T {
	t := T{}
	return &t
}

// CNIConfigData returns a cni network configuration, like
//
//	{
//	   "cniVersion": "0.3.0",
//	   "name": "net1",
//	   "type": "bridge",
//	   "bridge": "obr_net1",
//	   "isGateway": true,
//	   "ipMasq": false,
//	   "ipam": {
//	       "type": "host-local",
//	       "subnet": "10.23.0.0/26",
//	       "routes": [
//	           {
//	               "dst": "0.0.0.0/0"
//	           },
//	           {
//	               "dst": "10.23.0.0/24",
//	               "gw": "10.23.0.1"
//	           }
//	       ]
//	   }
//	}
func (t *T) CNIConfigData() (interface{}, error) {
	name := t.Name()
	nwStr := t.Network()
	brName := t.brName()
	subnetStr := t.subnet()
	brIP, err := t.bridgeIP()
	if err != nil {
		return nil, err
	}
	m := map[string]interface{}{
		"cniVersion": network.CNIVersion,
		"name":       name,
		"type":       "bridge",
		"bridge":     brName,
		"isGateway":  true,
		"ipMasq":     false,
		"ipam": map[string]interface{}{
			"type": "host-local",
			"routes": []map[string]interface{}{
				{"dst": defaultRouteDst(nwStr)},
				{"dst": nwStr, "gw": brIP.String()},
			},
			"subnet": subnetStr,
		},
	}
	return m, nil
}

func defaultRouteDst(cidr string) string {
	if isIP6(cidr) {
		return "::/0"
	} else {
		return "0.0.0.0/0"
	}
}

func isIP6(cidr string) bool {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return ip.To4() == nil
}

func (t *T) bridgeIP() (net.IP, error) {
	subnetStr := t.subnet()
	if subnetStr == "" {
		return nil, fmt.Errorf("network#%s.subnet is required", t.Name())
	}
	ip, _, err := net.ParseCIDR(subnetStr)
	if err != nil {
		return nil, err
	}
	ip[len(ip)-1]++
	return ip, nil
}

func ipsToMaskOnes(requestedIPs uint64, totalBits int) (int, error) {
	if requestedIPs == 0 {
		return totalBits, nil
	}

	// Find the next highest power of 2.
	// Example: 250 IPs -> 250-1 = 249. Binary length of 249 is 8 bits. 2^8 = 256.
	hostBits := bits.Len64(requestedIPs - 1)

	if hostBits > totalBits {
		return 0, fmt.Errorf("requested allocation size of %d IPs exceeds maximum %d-bit address space", requestedIPs, totalBits)
	}

	ones := totalBits - hostBits
	return ones, nil
}

func (t *T) allocateSubnets() error {
	subnetMap, err := t.subnets()
	if err != nil {
		return err
	}
	for _, s := range subnetMap {
		if s != "" {
			// A subnet setup is already configured. Don't change, even if bogus.
			return nil
		}
	}

	ip, netwk, err := net.ParseCIDR(t.Network())
	if err != nil {
		return err
	}
	ones, bits := netwk.Mask.Size()

	maskPerNode := t.GetInt("mask_per_node")
	if maskPerNode <= 0 {
		ipsPerNode := t.GetInt("ips_per_node")
		if ipsPerNode <= 0 {
			return fmt.Errorf("either mask_per_node or ips_per_node must be set to a value greater than 0")
		}
		maskPerNode, err = ipsToMaskOnes(uint64(ipsPerNode), bits)
		if err != nil {
			return err
		}
	}

	nodes, err := t.Nodes()
	if err != nil {
		return err
	}

	if err := t.checkMaxIpsPerNode(netwk, maskPerNode, nodes); err != nil {
		return err
	}

	m := make(map[string]string)

	for i, nodename := range nodes {
		nodeIp, subnet, err := computeSubnet(ip, int64(i), maskPerNode, ones, bits)
		if err != nil {
			return err
		}
		_ = nodeIp
		if err := t.Set("subnet@"+nodename, subnet); err != nil {
			return err
		}
		m[nodename] = subnet
		t.Log().Infof("assign subnet %s to node %s", subnet, nodename)
	}

	t.subnetMap = m
	return nil
}

func computeSubnet(baseIP net.IP, nodeIndex int64, maskPerNode int, ones, bits int) (net.IP, string, error) {
	ip16 := baseIP.To16()
	if ip16 == nil {
		return nil, "", fmt.Errorf("invalid base ip %s", baseIP)
	}

	if maskPerNode > bits || maskPerNode < 0 {
		return nil, "", fmt.Errorf("maskPerNode /%d out of bounds for %d-bit space", maskPerNode, bits)
	}
	hostBits := bits - maskPerNode
	ipsPerNode := new(big.Int).Lsh(big.NewInt(1), uint(hostBits))

	// Compute the offset block: nodeIndex * ipsPerNode
	baseInt := new(big.Int).SetBytes(ip16)
	idxBig := big.NewInt(nodeIndex)
	offset := new(big.Int).Mul(idxBig, ipsPerNode)

	// Add the offset to the base address
	nodeInt := new(big.Int).Add(baseInt, offset)

	parentHostBits := bits - ones
	parentSize := new(big.Int).Lsh(big.NewInt(1), uint(parentHostBits))
	parentCeiling := new(big.Int).Add(baseInt, parentSize)

	if nodeInt.Cmp(parentCeiling) >= 0 {
		return nil, "", fmt.Errorf("computed subnet index %d overflows the parent network boundary", nodeIndex)
	}

	// Unpack bytes back to net.IP
	nodeBytes := nodeInt.FillBytes(make([]byte, 16))
	nodeIP := net.IP(nodeBytes)

	if baseIP.To4() != nil {
		nodeIP = nodeIP.To4()
	}

	subnet := fmt.Sprintf("%s/%d", nodeIP.String(), maskPerNode)

	return nodeIP, subnet, nil
}

func (t *T) checkMaxIpsPerNode(network *net.IPNet, maskPerNode int, nodes []string) error {
	if network == nil || network.Mask == nil {
		return fmt.Errorf("invalid network context provided")
	}

	numNodes := int64(len(nodes))
	if numNodes == 0 {
		// Nothing to distribute
		return nil
	}

	ones, bits := network.Mask.Size()

	if maskPerNode < ones || maskPerNode > bits {
		return fmt.Errorf("requested node mask /%d is out of bounds for parent network %s", maskPerNode, network)
	}

	one := big.NewInt(1)

	maxIPs := new(big.Int).Lsh(one, uint(bits-ones))
	requestedIPsPerNode := new(big.Int).Lsh(one, uint(bits-maskPerNode))
	totalRequiredIPs := new(big.Int).Mul(requestedIPsPerNode, big.NewInt(numNodes))
	if totalRequiredIPs.Cmp(maxIPs) > 0 {
		maxIPsPerNodePossible := new(big.Int).Div(maxIPs, big.NewInt(numNodes))
		return fmt.Errorf("ips_per_node=%s (/%d) is too large. Total required across %d nodes is %s, but %s only provides %s total IPs (max possible per node is ~%s)",
			requestedIPsPerNode.String(),
			maskPerNode,
			numNodes,
			totalRequiredIPs.String(),
			network.String(),
			maxIPs.String(),
			maxIPsPerNodePossible.String(),
		)
	}
	return nil
}

func (t *T) Setup() error {
	var (
		localIP net.IP
		brIP    net.IP
		link    netlink.Link
		err     error
	)
	if err := t.allocateSubnets(); err != nil {
		return err
	}
	if brIP, err = t.bridgeIP(); err != nil {
		return err
	}
	if link, err = t.setupBridge(); err != nil {
		return fmt.Errorf("setup br: %w", err)
	}
	if err := t.setupBridgeIP(link, brIP); err != nil {
		return fmt.Errorf("setup br ip: %w", err)
	}
	if err := t.setupBridgeMAC(link, brIP); err != nil {
		return fmt.Errorf("setup mac: %w", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("link up: %w", err)
	}
	if localIP, err = t.getLocalIP(); err != nil {
		return fmt.Errorf("get local ip: %w", err)
	}
	nodes, err := t.Nodes()
	if err != nil {
		return err
	}
	for idx, nodename := range nodes {
		if err := t.setupNode(nodename, idx, localIP, brIP); err != nil {
			return fmt.Errorf("setup network to node %s: %w", nodename, err)
		}
	}
	return nil
}

func (t *T) setupNode(nodename string, nodeIndex int, localIP, brIP net.IP) error {
	var route network.Route
	tunnel := t.tunnel()
	peerIP, err := t.getNodeIP(nodename)
	if err != nil {
		return fmt.Errorf("get peer ip: %w", err)
	}
	subnet, ok := t.subnetMap[nodename]
	if !ok {
		return fmt.Errorf("no peer subnet: %w", err)
	}
	_, dst, err := net.ParseCIDR(subnet)
	if err != nil {
		return fmt.Errorf("parse peer subnet %s: %w", subnet, err)
	}
	if hostname.Hostname() == nodename {
		route = network.Route{
			Nodename: nodename,
			Dst:      dst,
			Dev:      t.brName(),
		}
	} else if v, err := t.mustTunnel(tunnel, peerIP); err != nil {
		return fmt.Errorf("must tunnel: %w", err)
	} else if v {
		name := tunName(peerIP, nodeIndex)
		if err := t.setupNodeTunnelLink(nodename, name, localIP, peerIP); err != nil {
			return fmt.Errorf("setup tunnel: %w", err)
		}
		if err := t.setupNodeTunnelLinkUp(name); err != nil {
			return fmt.Errorf("setup tunnel: %w", err)
		}
		route = network.Route{
			Nodename: nodename,
			Dst:      dst,
			Dev:      name,
			Src:      brIP,
		}
	} else {
		route = network.Route{
			Nodename: nodename,
			Dst:      dst,
			Gateway:  peerIP,
		}
	}
	if route.Gateway == nil && route.Dev == "" {
		t.Log().Infof("skip route setup because no gateway and no dev of node %s", nodename)
		return nil
	}
	if err := t.setupNodeRoutes(route); err != nil {
		return fmt.Errorf("setup route: %w", err)
	}
	return nil
}

func (t *T) mustTunnel(tunnel string, peerIP net.IP) (bool, error) {
	if tunnel == "never" {
		return false, nil
	}
	if tunnel == "always" {
		return true, nil
	}
	_, ipnet, err := network.IPReachableFrom(peerIP)
	if err != nil {
		return false, err
	}
	if ipnet != nil {
		t.Log().Tracef("%s is reachable from %s. tunnel not needed", peerIP, ipnet)
		return false, nil
	}
	return true, nil
}

func (t *T) setupNodeTunnelLink(nodename, name string, localIP, peerIP net.IP) error {
	mode := t.GetString("tunnel_mode")
	if mode == "ipip" && localIP.To4() == nil {
		mode = "ip6ip6"
	}

	// clean up existing tunnels with same endpoints but different name
	link, err := t.getTunnelByEndpoints(localIP, peerIP)
	if err != nil {
		return fmt.Errorf("get tunnel from %s to %s: %w", localIP, peerIP, err)
	}
	if link != nil {
		if link.Attrs().Name == name {
			if !t.isSameTunnelMode(link, mode) {
				err := netlink.LinkDel(link)
				if err != nil {
					return fmt.Errorf("del tunnel to %s: %w", nodename, err)
				}
				err = t.addTunnel(name, mode, localIP, peerIP)
				if err != nil {
					return fmt.Errorf("add tunnel to %s: %w", nodename, err)
				}
				return nil

			}
			t.Log().Infof("tunnel to %s is already setup", nodename)
			return nil
		} else {
			t.Log().Infof("delete conflicting tunnel %s from %s to %s", link.Attrs().Name, localIP, peerIP)
			if err := netlink.LinkDel(link); err != nil {
				return err
			}
		}
	}

	// modify up existing tunnels with same name but different endpoints
	// or add a new tunnel
	link, err = netlink.LinkByName(name)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); !ok {
			return err
		}
	}
	if link == nil {
		err := t.addTunnel(name, mode, localIP, peerIP)
		if err != nil {
			return fmt.Errorf("add tunnel to %s: %w", nodename, err)
		}
		return nil
	}
	if !t.isSameTunnelMode(link, mode) {
		err := netlink.LinkDel(link)
		if err != nil {
			return fmt.Errorf("del tunnel to %s: %w", nodename, err)
		}
		err = t.addTunnel(name, mode, localIP, peerIP)
		if err != nil {
			return fmt.Errorf("add tunnel to %s: %w", nodename, err)
		}
		return nil

	}
	if !t.isSameTunnelEndpoints(link, localIP, peerIP) {
		err := t.modTunnel(name, mode, localIP, peerIP)
		if err != nil {
			return fmt.Errorf("modify tunnel to %s: %w", nodename, err)
		}
		return nil
	}
	t.Log().Infof("tunnel to %s is already configured", nodename)
	return nil
}

func (t *T) setupNodeTunnelLinkUp(name string) error {
	if link, err := netlink.LinkByName(name); err != nil {
		return fmt.Errorf("link up: %w", err)
	} else if link != nil {
		t.Log().Infof("link up %s", name)
		netlink.LinkSetUp(link)
	}

	return nil
}

func (t *T) getTunnelByEndpoints(localIP, peerIP net.IP) (netlink.Link, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		var local, remote net.IP
		switch tun := link.(type) {
		case *netlink.Iptun:
			local = tun.Local
			remote = tun.Remote
		case *netlink.Ip6tnl:
			local = tun.Local
			remote = tun.Remote
		}
		if local.Equal(localIP) && remote.Equal(peerIP) {
			return link, nil
		}
	}
	return nil, nil
}

func (t *T) isSameTunnelMode(link netlink.Link, mode string) bool {
	name := link.Attrs().Name
	switch link.(type) {
	case *netlink.Gretun:
		if mode != "gre" {
			t.Log().Infof("%s mode is gre, expected %s", name, mode)
			return false
		}
	case *netlink.Iptun:
		if mode != "ipip" {
			t.Log().Infof("%s mode is ipip, expected %s", name, mode)
			return false
		}
	case *netlink.Ip6tnl:
		if mode != "ip6ip6" {
			t.Log().Infof("%s mode is ip6ip6, expected %s", name, mode)
			return false
		}
	}
	t.Log().Infof("%s mode is already %s", name, mode)
	return true
}

func (t *T) isSameTunnelEndpoints(link netlink.Link, localIP, peerIP net.IP) bool {
	name := link.Attrs().Name
	var local, remote net.IP
	switch tun := link.(type) {
	case *netlink.Gretun:
		local = tun.Local
		remote = tun.Remote
	case *netlink.Iptun:
		local = tun.Local
		remote = tun.Remote
	case *netlink.Ip6tnl:
		local = tun.Local
		remote = tun.Remote
	default:
		return false
	}
	if !local.Equal(localIP) {
		t.Log().Infof("tunnel %s local ip is %s, should be %s", name, local, localIP)
		return false
	}
	if !remote.Equal(peerIP) {
		t.Log().Infof("tunnel %s remote ip is %s, should be %s", name, remote, peerIP)
		return false
	}
	t.Log().Infof("tunnel %s endpoints are already %s to %s", name, local, remote)
	return true
}

func (t *T) modTunnel(name, mode string, localIP, peerIP net.IP) error {
	if mode == "gre" {
		return t.modTunnelGre(name, localIP, peerIP)
	}
	if localIP.To4() == nil {
		return t.modTunnelIp6(name, localIP, peerIP)
	} else {
		return t.modTunnelIp4(name, localIP, peerIP)
	}
}

func (t *T) addTunnel(name, mode string, localIP, peerIP net.IP) error {
	if mode == "gre" {
		return t.addTunnelGre(name, localIP, peerIP)
	}
	if localIP.To4() == nil {
		return t.addTunnelIp6(name, localIP, peerIP)
	} else {
		return t.addTunnelIp4(name, localIP, peerIP)
	}
}

func (t *T) loggerWithLink(link any) *plog.Logger {
	return t.Log().Attr("link", link)
}

func (t *T) modTunnelGre(name string, localIP, peerIP net.IP) error {
	link := &netlink.Ip6tnl{
		LinkAttrs: netlink.LinkAttrs{
			Name:      name,
			EncapType: "gretun",
		},
		Local:  localIP,
		Remote: peerIP,
	}
	t.loggerWithLink(link).Infof("modify gre tun %s", name)
	if h, err := netlink.NewHandle(); err != nil {
		defer h.Delete()
		return h.LinkModify(link)
	} else {
		return err
	}
}

func (t *T) modTunnelIp6(name string, localIP, peerIP net.IP) error {
	link := &netlink.Ip6tnl{
		LinkAttrs: netlink.LinkAttrs{
			Name:      name,
			EncapType: "ip6tnl",
		},
		Local:  localIP,
		Remote: peerIP,
	}
	t.loggerWithLink(link).Infof("modify ipip tun %s", name)
	if h, err := netlink.NewHandle(); err != nil {
		defer h.Delete()
		return h.LinkModify(link)
	} else {
		return err
	}
}

func (t *T) modTunnelIp4(name string, localIP, peerIP net.IP) error {
	link := &netlink.Iptun{
		LinkAttrs: netlink.LinkAttrs{
			Name:      name,
			EncapType: "ipip",
		},
		Local:  localIP,
		Remote: peerIP,
	}
	t.loggerWithLink(link).Infof("modify ipip tun %s", name)
	if h, err := netlink.NewHandle(); err != nil {
		defer h.Delete()
		return h.LinkModify(link)
	} else {
		return err
	}
}

func (t *T) addTunnelGre(name string, localIP, peerIP net.IP) error {
	link := &netlink.Gretun{
		LinkAttrs: netlink.LinkAttrs{
			Name:      name,
			EncapType: "gretun",
		},
		Local:  localIP,
		Remote: peerIP,
	}
	t.loggerWithLink(link).Infof("add gre tun %s", name)
	return netlink.LinkAdd(link)
}

func (t *T) addTunnelIp6(name string, localIP, peerIP net.IP) error {
	link := &netlink.Ip6tnl{
		LinkAttrs: netlink.LinkAttrs{
			Name:      name,
			EncapType: "ip6tnl",
		},
		Local:  localIP,
		Remote: peerIP,
	}
	t.loggerWithLink(link).Infof("add ipip tun %s", name)
	return netlink.LinkAdd(link)
}

func (t *T) addTunnelIp4(name string, localIP, peerIP net.IP) error {
	link := &netlink.Iptun{
		LinkAttrs: netlink.LinkAttrs{
			Name:      name,
			EncapType: "ipip",
		},
		Local:  localIP,
		Remote: peerIP,
	}
	t.loggerWithLink(link).Infof("add ipip tun %s", name)
	return netlink.LinkAdd(link)
}

func tunName(peerIP net.IP, nodeIndex int) string {
	if peerIP.To4() == nil {
		return fmt.Sprintf("otun%d", nodeIndex)
	} else {
		return fmt.Sprintf("tun%s", strings.ReplaceAll(peerIP.String(), ".", ""))
	}
}

func (t *T) setupBridge() (netlink.Link, error) {
	la := netlink.NewLinkAttrs()
	la.Name = t.brName()
	link, err := netlink.LinkByName(la.Name)
	_, linkNotFound := err.(netlink.LinkNotFoundError)
	switch {
	case linkNotFound:
	case err != nil:
		return nil, err
	case link != nil:
		t.Log().Infof("bridge link %s already exists", la.Name)
		return link, nil
	}
	br := &netlink.Bridge{LinkAttrs: la}
	err = netlink.LinkAdd(br)
	if err != nil {
		return nil, fmt.Errorf("failed to add bridge link %s: %v", la.Name, err)
	}
	t.Log().Infof("added bridge link %s", la.Name)
	return br, nil
}

func (t *T) subnet() string {
	if t.subnetMap != nil {
		s, _ := t.subnetMap[hostname.Hostname()]
		return s
	}
	return t.GetString("subnet")
}

func (t *T) subnets() (map[string]string, error) {
	if t.subnetMap != nil {
		return t.subnetMap, nil
	}
	m := make(map[string]string)
	nodes, err := t.Nodes()
	if err != nil {
		return nil, err
	}
	for _, nodename := range nodes {
		m[nodename] = t.GetStringAs("subnet", nodename)
	}
	t.subnetMap = m
	return m, nil
}

func (t *T) tunnel() string {
	return t.GetString("tunnel")
}

func (t *T) brName() string {
	if s := t.GetString("dev"); s != "" {
		return s
	}
	return "obr_" + t.Name()
}

func (t *T) BackendDevName() string {
	if v := t.GetBool("public"); v {
		return ""
	}
	return t.brName()
}

func (t *T) setupBridgeMAC(br netlink.Link, brIP net.IP) error {
	var (
		mac net.HardwareAddr
		err error
	)
	if br == nil {
		return nil
	}
	if t.IsIP6() {
		return nil
	}
	if mac, err = network.MACFromIP4(brIP); err != nil {
		return err
	}
	if br.Attrs().HardwareAddr.String() == mac.String() {
		t.Log().Infof("bridge %s mac is already %s", br.Attrs().Name, mac)
		return nil
	}
	t.Log().Infof("bridge %s set mac to %s", br.Attrs().Name, mac)
	return netlink.LinkSetHardwareAddr(br, mac)
}

func (t *T) setupBridgeIP(br netlink.Link, brIP net.IP) error {
	subnetStr := t.subnet()
	if br == nil {
		return nil
	}
	brName := t.brName()
	_, ipnet, err := net.ParseCIDR(subnetStr)
	if err != nil {
		return err
	}
	ipnet.IP = brIP
	ipnetStr := ipnet.String()

	if intf, err := net.InterfaceByName(brName); err != nil {
		return err
	} else if addrs, err := intf.Addrs(); err != nil {
		return err
	} else {
		for _, addr := range addrs {
			if addr.String() == ipnetStr {
				t.Log().Infof("bridge ip %s already added to %s", ipnet, brName)
				return nil
			}
		}
	}
	addr := &netlink.Addr{IPNet: ipnet}
	if err := netlink.AddrAdd(br, addr); err != nil {
		return err
	}
	t.Log().Infof("added ip %s to bridge %s", ipnet, brName)
	return nil
}

// getNodeIP returns the addr scoped for nodename from the network config.
// Defaults to the first resolved ip address with the network address family (ip4 or ip6).
func (t *T) getNodeIP(nodename string) (net.IP, error) {
	var addr string
	if nodename == hostname.Hostname() {
		addr = t.GetString("addr")
	} else {
		addr = t.GetStringAs("addr", nodename)
	}
	if addr != "" {
		ip := net.ParseIP(addr)
		if ip != nil {
			return ip, nil
		}
		af := t.getAF()
		return network.GetNodeAddr(addr, af)
	}
	af := t.getAF()
	return network.GetNodeAddr(nodename, af)
}

// getLocalIP returns the addr set in the network config.
// Defaults to the first resolved ip address with the network address family (ip4 or ip6).
func (t *T) getLocalIP() (net.IP, error) {
	return t.getNodeIP(hostname.Hostname())
}

// getAF returns the network address family (ip4 or ip6).
func (t *T) getAF() (af string) {
	if t.IsIP6() {
		af = "ip6"
	} else {
		af = "ip4"
	}
	return
}

func (t *T) setupNodeRoutes(route network.Route) error {
	for _, table := range t.Tables() {
		route.Table = table
		t.Log().Infof("route add %s", route)
		if err := route.Add(); err != nil {
			return fmt.Errorf("route add %s: %w", route, err)
		}
	}
	return nil
}
