//go:build linux

package resipcni

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords of an ip.cni resource beside the dns group of
// the ip drivers. They have the ids, names and orders of the addressing and
// interface groups of ip.netns, but none of the strategies of its
// addressing: the address of a cni resource is always the one the ipam of
// its network hands out.
var (
	// GroupAddressing holds the network the address is drawn from.
	GroupAddressing = keywords.Group{
		ID:    "addressing",
		Name:  "Addressing",
		Text:  keywords.NewText(fs, "text/group/addressing"),
		Order: 10,
	}

	// GroupInterface holds the network namespace the address is plumbed
	// in, and the interface it has there.
	GroupInterface = keywords.Group{
		ID:    "interface",
		Name:  "Interface",
		Text:  keywords.NewText(fs, "text/group/interface"),
		Order: 20,
	}
)
