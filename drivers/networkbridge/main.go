package networkbridge

import (
	"fmt"
	"net"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/network"
	"github.com/vishvananda/netlink"
)

type (
	T struct {
		network.T
	}
)

var (
	drvID = driver.NewID(driver.GroupNetwork, "bridge")
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

func (t *T) brName() string {
	return "obr_" + t.Name()
}

func (t *T) BackendDevName() string {
	return t.brName()
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

// CNIConfigData returns a cni network configuration, like
//
//	{
//	  "bridge": "cni0",
//	  "cniVersion": "0.3.0",
//	  "ipMasq": false,
//	  "name": "mynet",
//	  "ipam": {
//	    "routes": [
//	      {"dst": "0.0.0.0/0"}
//	    ],
//	    "subnet": "10.22.0.0/16",
//	    "type": "host-local"
//	  },
//	  "isGateway": true,
//	  "type": "bridge"
//	}
func (t *T) CNIConfigData() (interface{}, error) {
	nwStr := t.Network()
	brIP, err := t.bridgeIP()
	if err != nil {
		return nil, err
	}
	m := map[string]interface{}{
		"cniVersion": network.CNIVersion,
		"name":       t.Name(),
		"type":       "bridge",
		"bridge":     t.brName(),
		"isGateway":  true,
		"ipMasq":     false,
		"ipam": map[string]interface{}{
			"type": "host-local",
			"routes": []map[string]interface{}{
				{"dst": defaultRouteDst(nwStr)},
				{"dst": nwStr, "gw": brIP.String()},
			},
			"subnet": t.Network(),
		},
	}
	return m, nil
}

func (t *T) bridgeIP() (net.IP, error) {
	subnetStr := t.Network()
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

func (t *T) setupBridgeIP(br netlink.Link, brIP net.IP) error {
	subnetStr := t.Network()
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

func (t *T) IsIP6() bool {
	ip, _, err := net.ParseCIDR(t.Network())
	if err != nil {
		return false
	}
	return ip.To4() == nil
}

func (t *T) Setup() error {
	var (
		brIP net.IP
		link netlink.Link
		err  error
	)
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
	return nil
}
