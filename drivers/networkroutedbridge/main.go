package networkroutedbridge

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"opensvc.com/opensvc/core/network"
	"opensvc.com/opensvc/core/object"
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
	brName := "obr_" + name
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
			"subnet": t.GetString("subnet"),
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
	subnetStr := t.GetString("subnet")
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
	return nil
}

func (t T) setupBridge(n *object.Node) error {
	la := netlink.NewLinkAttrs()
	la.Name = "obr_" + t.Name()
	if intf, err := net.InterfaceByName(la.Name); err != nil {
		return err
	} else if intf != nil {
		n.Log().Info().Msgf("bridge link %s already exists", la.Name)
		return nil
	}
	br := &netlink.Bridge{LinkAttrs: la}
	err := netlink.LinkAdd(br)
	if err != nil {
		return fmt.Errorf("could not add bridge link %s: %v", la.Name, err)
	}
	n.Log().Info().Msgf("added bridge link %s")
	return nil
}
