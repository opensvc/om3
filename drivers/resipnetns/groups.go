//go:build linux

package resipnetns

import (
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/drivers/resip"
)

// The groups of the keywords of an ip.netns resource beside the addressing
// and dns groups of the ip drivers.
//
// The mode is no selector of strategies of the interface group: the
// vlan_tag and vlan_mode keywords only the ovs mode reads already depend on
// it, the macaddr keyword is read by the bridge and ovs modes alone but
// ignored by the others rather than conflicting with them, and a mode left
// unset is the first one of the network the address is drawn from, which
// the validation of a configuration does not know.
var (
	// GroupAddressing is the addressing group of the ip drivers, as an
	// ip.netns reads it: a static address may name a network too, which
	// then says the device it is plumbed on, and the netmask and the
	// gateway the configuration does not set. On a layer 2 network, which
	// om draws no address from, it is the only way to hold one.
	//
	// So the static strategy does not exclude the network, and its device
	// is not required of a section naming one.
	GroupAddressing = keywords.Group{
		ID:       resip.GroupAddressing.ID,
		Name:     resip.GroupAddressing.Name,
		Text:     keywords.NewText(fs, "text/group/addressing"),
		Order:    resip.GroupAddressing.Order,
		Required: resip.GroupAddressing.Required,
		Strategies: []keywords.Strategy{
			{
				ID:             "static",
				Name:           "Static address",
				Text:           keywords.NewText(fs, "text/strategy/addressing/static"),
				Uses:           []string{"name"},
				Excludes:       []string{"addr"},
				Requires:       []string{"dev"},
				RequiresUnless: []string{"network"},
			},
			{
				ID:       "ipam",
				Name:     "Address drawn from a network",
				Text:     keywords.NewText(fs, "text/strategy/addressing/ipam"),
				Uses:     []string{"network", "addr"},
				Excludes: []string{"name"},
			},
		},
	}

	// GroupInterface holds the network namespace the address is plumbed
	// in, the interface it has there, and how that interface is linked to
	// the device of the node.
	GroupInterface = keywords.Group{
		ID:    "interface",
		Name:  "Interface",
		Text:  keywords.NewText(fs, "text/group/interface"),
		Order: 20,
	}

	// GroupRouting holds the routes the resource sets in the network
	// namespace: its default route, and the route to the network of the
	// address the kernel adds, which it may remove.
	GroupRouting = keywords.Group{
		ID:    "routing",
		Name:  "Routing",
		Text:  keywords.NewText(fs, "text/group/routing"),
		Order: 30,
	}
)
