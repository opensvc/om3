package object

import (
	"fmt"
	"net"
	"strconv"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/util/converters"
)

// The keywords of the l2_bridge networks: a bridge with no address on each
// node, meshed with the ones of the other nodes by the ports of a lan they
// share or by a vxlan, the strategies of the mesh group, and its options.

// validateIntRange returns a check of a value as an integer from min to max.
func validateIntRange(min, max int) func(string) error {
	return func(s string) error {
		i, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("%s is not an integer", s)
		}
		if i < min || i > max {
			return fmt.Errorf("%d is not from %d to %d", i, min, max)
		}
		return nil
	}
}

// validateMAC refuses a value that is no hardware address.
func validateMAC(s string) error {
	_, err := net.ParseMAC(s)
	return err
}

var (
	nodeGroupMesh = keywords.Group{
		ID:    "mesh",
		Name:  "Mesh",
		Text:  keywords.NewText(fs, "text/group/node/mesh"),
		Order: 25,
	}

	// nodeGroupL2BridgeMesh is the mesh group of a l2_bridge network: the
	// bridges of the nodes share a lan, or a vxlan. Neither strategy
	// followed, the bridge is alone on its node, so the group is not
	// required.
	nodeGroupL2BridgeMesh = keywords.Group{
		ID:    nodeGroupMesh.ID,
		Name:  nodeGroupMesh.Name,
		Text:  nodeGroupMesh.Text,
		Order: nodeGroupMesh.Order,
		Strategies: []keywords.Strategy{
			{
				ID:       "ports",
				Name:     "Through a shared lan",
				Text:     keywords.NewText(fs, "text/strategy/node/l2_bridge/ports"),
				Uses:     []string{"ports", "vlan"},
				Excludes: []string{"vxlan_vni", "vxlan_port", "vxlan_addr"},
			},
			{
				ID:       "vxlan",
				Name:     "Through a vxlan",
				Text:     keywords.NewText(fs, "text/strategy/node/l2_bridge/vxlan"),
				Uses:     []string{"vxlan_vni", "vxlan_port", "vxlan_addr"},
				Excludes: []string{"ports", "vlan"},
				Requires: []string{"vxlan_vni"},
			},
		},
	}

	kwNodeNetworkL2BridgePorts = keywords.Keyword{
		Converter: converters.List,
		Example:   "eno2",
		Option:    "ports",
		Group:     nodeGroupMesh.ID,
		Section:   "network",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/node/network.l2_bridge.ports"),
		Types:     []string{"l2_bridge"},
	}
	kwNodeNetworkL2BridgeVLAN = keywords.Keyword{
		Converter: converters.Int,
		Example:   "100",
		Option:    "vlan",
		Group:     nodeGroupMesh.ID,
		Section:   "network",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/node/network.l2_bridge.vlan"),
		Types:     []string{"l2_bridge"},
		Validate:  validateIntRange(1, 4094),
	}
	kwNodeNetworkL2BridgeVXLANVNI = keywords.Keyword{
		Converter: converters.Int,
		Example:   "1001",
		Option:    "vxlan_vni",
		Group:     nodeGroupMesh.ID,
		Section:   "network",
		Text:      keywords.NewText(fs, "text/kw/node/network.l2_bridge.vxlan_vni"),
		Types:     []string{"l2_bridge"},
		Validate:  validateIntRange(1, 16777215),
	}
	kwNodeNetworkL2BridgeVXLANPort = keywords.Keyword{
		Converter: converters.Int,
		Default:   "4789",
		Option:    "vxlan_port",
		Group:     nodeGroupMesh.ID,
		Section:   "network",
		Text:      keywords.NewText(fs, "text/kw/node/network.l2_bridge.vxlan_port"),
		Types:     []string{"l2_bridge"},
		Validate:  validateIntRange(1, 65535),
	}
	kwNodeNetworkL2BridgeVXLANAddr = keywords.Keyword{
		Example:  "10.0.0.1",
		Option:   "vxlan_addr",
		Group:    nodeGroupMesh.ID,
		Section:  "network",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/node/network.l2_bridge.vxlan_addr"),
		Types:    []string{"l2_bridge"},
	}
	kwNodeNetworkL2BridgeMTU = keywords.Keyword{
		Converter: converters.Int,
		Example:   "1450",
		Option:    "mtu",
		Group:     nodeGroupInterface.ID,
		Section:   "network",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/node/network.l2_bridge.mtu"),
		Types:     []string{"l2_bridge"},
		Validate:  validateIntRange(68, 65535),
	}
	kwNodeNetworkL2BridgeSTP = keywords.Keyword{
		Converter: converters.Bool,
		Option:    "stp",
		Group:     nodeGroupInterface.ID,
		Section:   "network",
		Text:      keywords.NewText(fs, "text/kw/node/network.l2_bridge.stp"),
		Types:     []string{"l2_bridge"},
	}
	kwNodeNetworkL2BridgeHairpin = keywords.Keyword{
		Converter: converters.Bool,
		Option:    "hairpin",
		Group:     nodeGroupInterface.ID,
		Section:   "network",
		Text:      keywords.NewText(fs, "text/kw/node/network.l2_bridge.hairpin"),
		Types:     []string{"l2_bridge"},
	}
	kwNodeNetworkL2BridgeMcastSnooping = keywords.Keyword{
		Converter: converters.Bool,
		Option:    "mcast_snooping",
		Group:     nodeGroupInterface.ID,
		Section:   "network",
		Text:      keywords.NewText(fs, "text/kw/node/network.l2_bridge.mcast_snooping"),
		Types:     []string{"l2_bridge"},
	}
	kwNodeNetworkL2BridgeAgeingTime = keywords.Keyword{
		Converter: converters.Duration,
		Example:   "5m",
		Option:    "ageing_time",
		Group:     nodeGroupInterface.ID,
		Section:   "network",
		Text:      keywords.NewText(fs, "text/kw/node/network.l2_bridge.ageing_time"),
		Types:     []string{"l2_bridge"},
	}
	kwNodeNetworkL2BridgeMAC = keywords.Keyword{
		Example:  "02:00:00:00:00:01",
		Option:   "mac",
		Group:    nodeGroupInterface.ID,
		Section:  "network",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/node/network.l2_bridge.mac"),
		Types:    []string{"l2_bridge"},
		Validate: validateMAC,
	}
)
