//go:build linux

package resipnetns

import (
	"github.com/opensvc/om3/v3/core/keywords"
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
