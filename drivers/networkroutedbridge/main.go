package networkroutedbridge

import (
	"fmt"
	"net"
	"strings"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"opensvc.com/opensvc/core/network"
	"opensvc.com/opensvc/util/hostname"
)

type (
	T struct {
		network.T
	}
)

func init() {
	network.Register("routed_bridge", NewNetworker)
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

func (t T) Usage() (network.StatusUsage, error) {
	usage := network.StatusUsage{}
	return usage, nil
}

// CNIConfigData returns a cni network configuration, like
// {
//    "cniVersion": "0.3.0",
//    "name": "net1",
//    "type": "bridge",
//    "bridge": "obr_net1",
//    "isGateway": true,
//    "ipMasq": false,
//    "ipam": {
//        "type": "host-local",
//        "subnet": "10.23.0.0/26",
//        "routes": [
//            {
//                "dst": "0.0.0.0/0"
//            },
//            {
//                "dst": "10.23.0.0/24",
//                "gw": "10.23.0.1"
//            }
//        ]
//    }
//}
func (t T) CNIConfigData() (interface{}, error) {
	name := t.Name()
	nwStr := t.Network()
	brName := t.brName()
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
				{"dst": nwStr, "gw": brIP},
			},
			"subnet": t.subnet(),
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

func (t T) bridgeIP() (string, error) {
	subnetStr := t.subnet()
	if subnetStr == "" {
		return "", fmt.Errorf("network#%s.subnet is required", t.Name())
	}
	ip, _, err := net.ParseCIDR(subnetStr)
	if err != nil {
		return "", err
	}
	ip[len(ip)-1]++
	return ip.String(), nil
}

func (t *T) Setup() error {
	if err := t.setupBridge(); err != nil {
		return err
	}
	if err := t.setupBridgeIP(); err != nil {
		return err
	}
	if err := t.setupTunnels(); err != nil {
		return err
	}
	if err := t.setupRoutes(); err != nil {
		return err
	}
	return nil
}

func isSameNetwork(localIP, peerIP net.IP) (bool, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false, err
	}
	for _, addr := range addrs {
		if ip, ipnet, _ := net.ParseCIDR(addr.String()); ip.Equal(localIP) {
			return ipnet.Contains(peerIP), nil
		}
	}
	return false, nil
}

func (t *T) setupTunnel(nodename string, localIP net.IP, af string, nodeIndex int, auto bool) error {
	if nodename == hostname.Hostname() {
		return nil
	}
	peerIP, err := t.getNodeIP(nodename, af)
	if err != nil {
		return err
	}
	if auto {
		if same, err := isSameNetwork(localIP, peerIP); err != nil {
			return err
		} else if same {
			t.Log().Debug().Msgf("%s (%s) and %s (%s) are in the same network. skip tunnel setup", hostname.Hostname(), localIP, nodename, peerIP)
			return nil
		}
	}
	name := tunName(peerIP, nodeIndex)

	link, err := netlink.LinkByName(name)
	switch {
	case err != nil:
		if _, ok := err.(netlink.LinkNotFoundError); !ok {
			return err
		}
		fallthrough
	case link == nil:
		if err := t.addTunnel(name, localIP, peerIP); err != nil {
			return errors.Wrapf(err, "add tunnel to %s", nodename)
		}
	case link != nil && t.isSameTunnel(link, localIP, peerIP):
		t.Log().Info().Msgf("preserve tunnel to %s: already configured", nodename)
		return nil
	default:
		if err := t.modTunnel(name, localIP, peerIP); err != nil {
			return errors.Wrapf(err, "modify tunnel to %s", nodename)
		}
	}

	return nil
}

func (t T) isSameTunnel(link netlink.Link, localIP, peerIP net.IP) bool {
	name := link.Attrs().Name
	var local, remote net.IP
	switch tun := link.(type) {
	case *netlink.Iptun:
		if localIP.To4() == nil {
			t.Log().Info().Msgf("link %s is not a ipip tunnel", name)
			return false
		}
		local = tun.Local
		remote = tun.Remote
	case *netlink.Ip6tnl:
		if localIP.To4() != nil {
			t.Log().Info().Msgf("link %s is not a ip6ip6 tunnel", name)
			return false
		}
		local = tun.Local
		remote = tun.Remote
	}
	if !local.Equal(localIP) {
		t.Log().Info().Msgf("tunnel %s local ip is %s, should be %s", name, local, localIP)
		return false
	}
	if !remote.Equal(peerIP) {
		t.Log().Info().Msgf("tunnel %s remote ip is %s, should be %s", name, remote, peerIP)
		return false
	}
	return true
}

func (t T) modTunnel(name string, localIP, peerIP net.IP) error {
	if localIP.To4() == nil {
		return t.modTunnel6(name, localIP, peerIP)
	} else {
		return t.modTunnel4(name, localIP, peerIP)
	}
}

func (t T) addTunnel(name string, localIP, peerIP net.IP) error {
	if localIP.To4() == nil {
		return t.addTunnel6(name, localIP, peerIP)
	} else {
		return t.addTunnel4(name, localIP, peerIP)
	}
}

func (t T) modTunnel6(name string, localIP, peerIP net.IP) error {
	link := &netlink.Ip6tnl{
		LinkAttrs: netlink.LinkAttrs{
			Name:      name,
			EncapType: "ip6tnl",
		},
		Local:  localIP,
		Remote: peerIP,
	}
	t.Log().Info().Interface("link", link).Msgf("modify ipip tun %s", name)
	if h, err := netlink.NewHandle(); err != nil {
		defer h.Delete()
		return h.LinkModify(link)
	} else {
		return err
	}
}

func (t T) modTunnel4(name string, localIP, peerIP net.IP) error {
	link := &netlink.Iptun{
		LinkAttrs: netlink.LinkAttrs{
			Name:      name,
			EncapType: "ipip",
		},
		Local:  localIP,
		Remote: peerIP,
	}
	t.Log().Info().Interface("link", link).Msgf("modify ipip tun %s", name)
	if h, err := netlink.NewHandle(); err != nil {
		defer h.Delete()
		return h.LinkModify(link)
	} else {
		return err
	}
}

func (t T) addTunnel6(name string, localIP, peerIP net.IP) error {
	link := &netlink.Ip6tnl{
		LinkAttrs: netlink.LinkAttrs{
			Name:      name,
			EncapType: "ip6tnl",
		},
		Local:  localIP,
		Remote: peerIP,
	}
	t.Log().Info().Interface("link", link).Msgf("add ipip tun %s", name)
	return netlink.LinkAdd(link)
}

func (t T) addTunnel4(name string, localIP, peerIP net.IP) error {
	link := &netlink.Iptun{
		LinkAttrs: netlink.LinkAttrs{
			Name:      name,
			EncapType: "ipip",
		},
		Local:  localIP,
		Remote: peerIP,
	}
	t.Log().Info().Interface("link", link).Msgf("add ipip tun %s", name)
	return netlink.LinkAdd(link)
}

func tunName(peerIP net.IP, nodeIndex int) string {
	if peerIP.To4() == nil {
		return fmt.Sprintf("otun%d", nodeIndex)
	} else {
		return fmt.Sprintf("tun%s", strings.ReplaceAll(peerIP.String(), ".", ""))
	}
}

func (t *T) setupTunnels() error {
	tunnel := t.tunnel()
	if tunnel == "never" {
		t.Log().Debug().Msg("skip tunnel setup: tunnel=never")
		return nil
	}
	auto := tunnel == "auto"
	af := t.getAF()
	localIP, err := t.getLocalIP(af)
	if err != nil {
		t.Log().Debug().Err(err).Msg("skip tunnel setup")
		return err
	}
	for idx, nodename := range t.Nodes() {
		if err := t.setupTunnel(nodename, localIP, af, idx, auto); err != nil {
			return errors.Wrapf(err, "setup tunnel to %s", nodename)
		}
	}
	return nil
}

func (t *T) setupRoutes() error {
	if l, err := t.Routes(); err != nil {
		t.Log().Err(err).Msg("setup routes")
		return err
	} else {
		return l.Add()
	}
}

func (t T) setupBridge() error {
	la := netlink.NewLinkAttrs()
	la.Name = t.brName()
	if intf, err := net.InterfaceByName(la.Name); err != nil {
		return err
	} else if intf != nil {
		t.Log().Info().Msgf("bridge link %s already exists", la.Name)
		return nil
	}
	br := &netlink.Bridge{LinkAttrs: la}
	err := netlink.LinkAdd(br)
	if err != nil {
		return fmt.Errorf("failed to add bridge link %s: %v", la.Name, err)
	}
	t.Log().Info().Msgf("added bridge link %s")
	return nil
}

func (t T) subnet() string {
	return t.GetString("subnet")
}

func (t T) subnetMap() map[string]string {
	m := make(map[string]string)
	for _, nodename := range t.Nodes() {
		m[nodename] = t.GetString("subnet@" + nodename)
	}
	return m
}

func (t T) tunnel() string {
	return t.GetString("tunnel")
}

func (t T) brName() string {
	return "obr_" + t.Name()
}

func (t T) setupBridgeIP() error {
	brIP, err := t.bridgeIP()
	brName := t.brName()
	br, err := netlink.LinkByName(brName)
	if err != nil {
		return err
	}
	if br == nil {
		return fmt.Errorf("bridge %s not found", brName)
	}

	subnetStr := t.subnet()
	_, ipnet, err := net.ParseCIDR(subnetStr)
	if err != nil {
		return err
	}
	ipnet.IP = net.ParseIP(brIP)
	ipnetStr := ipnet.String()

	if intf, err := net.InterfaceByName(brName); err != nil {
		return err
	} else if addrs, err := intf.Addrs(); err != nil {
		return err
	} else {
		for _, addr := range addrs {
			if addr.String() == ipnetStr {
				t.Log().Info().Msgf("bridge ip %s already added to %s", ipnet, brName)
				return nil
			}
		}
	}
	addr := &netlink.Addr{IPNet: ipnet}
	if err := netlink.AddrAdd(br, addr); err != nil {
		return err
	}
	t.Log().Info().Msgf("added ip %s to bridge %s", ipnet, brName)
	return nil
}

// getNodeIP returns the addr scoped for nodename from the network config.
// Defaults to the first resolved ip address with the network address family (ip4 or ip6).
func (t T) getNodeIP(nodename, af string) (net.IP, error) {
	var keyName string
	if nodename == hostname.Hostname() {
		keyName = "addr"
	} else {
		keyName = "addr@" + nodename
	}
	if addr := t.GetString(keyName); addr != "" {
		return net.ParseIP(addr), nil
	}
	return network.GetNodeAddr(nodename, af)
}

// getLocalIP returns the addr set in the network config.
// Defaults to the first resolved ip address with the network address family (ip4 or ip6).
func (t T) getLocalIP(af string) (net.IP, error) {
	return t.getNodeIP(hostname.Hostname(), af)
}

// getAF returns the network address family (ip4 or ip6).
func (t T) getAF() (af string) {
	if t.IsIP6() {
		af = "ip6"
	} else {
		af = "ip4"
	}
	return
}

func (t *T) Routes() (network.Routes, error) {
	routes := make(network.Routes, 0)
	af := t.getAF()
	for _, nodename := range t.Nodes() {
		if nodename == hostname.Hostname() {
			continue
		}
		for _, table := range t.Tables() {
			gw, err := t.getNodeIP(nodename, af)
			if err != nil {
				return routes, errors.Wrapf(err, "route to %s: gw", nodename)
			}
			dst, err := t.NodeSubnet(nodename)
			if err != nil {
				return routes, errors.Wrapf(err, "route to %s: dst", nodename)
			}
			if dst == nil {
				return routes, fmt.Errorf("route to %s: no dst subnet", nodename)
			}
			routes = append(routes, network.Route{
				Nodename: nodename,
				Dev:      t.brName(),
				Dst:      dst,
				Gateway:  gw,
				Table:    table,
			})
		}
	}
	t.Log().Debug().Interface("routes", routes).Msg("routes")
	return routes, nil
}
