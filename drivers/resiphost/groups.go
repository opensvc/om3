package resiphost

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords of an ip.host resource beside the addressing
// and dns groups of the ip drivers. They have the ids, names and orders of
// the interface and routing groups of ip.netns, their texts saying the
// interface of the node.
var (
	// GroupInterface holds the interface of the node the address is
	// plumbed on, and how.
	GroupInterface = keywords.Group{
		ID:    "interface",
		Name:  "Interface",
		Text:  keywords.NewText(fs, "text/group/interface"),
		Order: 20,
	}

	// GroupRouting holds the gateway of the address, which the driver
	// reads but sets no route with.
	GroupRouting = keywords.Group{
		ID:    "routing",
		Name:  "Routing",
		Text:  keywords.NewText(fs, "text/group/routing"),
		Order: 30,
	}
)
