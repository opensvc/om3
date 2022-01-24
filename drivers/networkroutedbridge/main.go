package networkroutedbridge

import (
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"opensvc.com/opensvc/core/network"
	"opensvc.com/opensvc/core/object"
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

func (t T) Setup(n *object.Node) error {
	if err := t.setupBridge(n); err != nil {
		return err
	}
	if err := t.setupBridgeIP(n); err != nil {
		return err
	}
	if err := t.setupRoutes(n); err != nil {
		return err
	}
	return nil
}

func (t T) setupRoutes(n *object.Node) error {
	if l, err := t.Routes(n); err != nil {
		n.Log().Err(err).Str("name", t.Name()).Msg("setup routes")
		return err
	} else {
		return l.Add()
	}
}

func (t T) setupBridge(n *object.Node) error {
	la := netlink.NewLinkAttrs()
	la.Name = t.brName()
	if intf, err := net.InterfaceByName(la.Name); err != nil {
		return err
	} else if intf != nil {
		n.Log().Info().Msgf("bridge link %s already exists", la.Name)
		return nil
	}
	br := &netlink.Bridge{LinkAttrs: la}
	err := netlink.LinkAdd(br)
	if err != nil {
		return fmt.Errorf("failed to add bridge link %s: %v", la.Name, err)
	}
	n.Log().Info().Msgf("added bridge link %s")
	return nil
}

func (t T) subnet() string {
	return t.GetString("subnet")
}

func (t T) brName() string {
	return "obr_" + t.Name()
}

func (t T) setupBridgeIP(n *object.Node) error {
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
				n.Log().Info().Msgf("%s already added to %s", ipnet, brName)
				return nil
			}
		}
	}
	addr := &netlink.Addr{IPNet: ipnet}
	if err := netlink.AddrAdd(br, addr); err != nil {
		return err
	}
	n.Log().Info().Msgf("added %s to %s", ipnet, brName)
	return nil
}

func (t T) Routes(n *object.Node) (network.Routes, error) {
	/*
		// getLocalIP returns the addr set in the network config.
		// Defaults to the first resolved ip address with the network address family (ip4 or ip6).
		getLocalIP := func(af string) (addr string, err error) {
			addr = t.GetString("addr")
			if addr != "" {
				return
			}
			addr, err = network.GetNodeAddr(hostname.Hostname(), af)
			return
		}
	*/

	// getAF returns the network address family (ip4 or ip6).
	getAF := func(nwStr string) (af string) {
		if t.IsIPv6() {
			af = "ip6"
		} else {
			af = "ip4"
		}
		return
	}

	// getGW returns the addr of the peer node set in the network config.
	// Defaults to the first resolved ip address with the network address family (ip4 or ip6).
	getGW := func(nodename, af string) (addr string, err error) {
		addr = t.GetString("addr@" + nodename)
		if addr != "" {
			return
		}
		addr, err = network.GetNodeAddr(nodename, af)
		return
	}

	routes := make(network.Routes, 0)
	nwStr := t.Network()
	af := getAF(nwStr)
	/*
		localIP, err := getLocalIP(af)
		if err != nil {
			return routes, err
		}
	*/
	for _, nodename := range n.Nodes() {
		if nodename == hostname.Hostname() {
			continue
		}
		for _, table := range t.Tables() {
			gw, err := getGW(nodename, af)
			if err != nil {
				return routes, errors.Wrapf(err, "route to %s: gw", nodename)
			}
			dst, err := t.NodeSubnet(nodename, n.Nodes())
			if err != nil {
				return routes, errors.Wrapf(err, "route to %s: dst", nodename)
			}
			if dst == nil {
				return routes, fmt.Errorf("route to %s: no dst subnet", nodename)
			}
			routes = append(routes, network.Route{
				Nodename: nodename,
				Dev:      t.brName(),
				Dst:      dst.String(),
				Gateway:  gw,
				Table:    table,
			})
		}
	}
	n.Log().Debug().Interface("routes", routes).Msg("routes")
	return routes, nil
}
