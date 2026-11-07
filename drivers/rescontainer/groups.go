package rescontainer

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords several container drivers have, declared once
// here and added by each driver whose keywords name them. A group holds, in
// a driver, the keywords that driver declares with its id: the network of an
// oci container is its netns and its resolver, the one of a kvm guest its
// bridges and om networks.
//
// The drivers add their own groups, as the image of an oci container, in
// the order between these.
var (
	// GroupIdentity holds the name of the container and its hostname.
	GroupIdentity = keywords.Group{
		ID:    "identity",
		Name:  "Identity",
		Text:  keywords.NewText(fs, "text/group/identity"),
		Order: 10,
	}

	// GroupMachine holds the virtual hardware of a virtual machine.
	GroupMachine = keywords.Group{
		ID:    "machine",
		Name:  "Virtual machine",
		Text:  keywords.NewText(fs, "text/group/machine"),
		Order: 20,
	}

	// GroupStorage holds where the container keeps its files, and the
	// storage of the node it uses.
	GroupStorage = keywords.Group{
		ID:    "storage",
		Name:  "Storage",
		Text:  keywords.NewText(fs, "text/group/storage"),
		Order: 50,
	}

	// GroupNetwork holds the networks the container is on, and its
	// resolver.
	GroupNetwork = keywords.Group{
		ID:    "network",
		Name:  "Network",
		Text:  keywords.NewText(fs, "text/group/network"),
		Order: 60,
	}

	// GroupGuest holds how the agent of the node reaches the system in the
	// container, and the agent running there, for the encapsulated
	// resources and the commands run in the container.
	GroupGuest = keywords.Group{
		ID:    "guest",
		Name:  "Guest",
		Text:  keywords.NewText(fs, "text/group/guest"),
		Order: 80,
	}

	// GroupTimeouts holds how long the actions on the container may take.
	GroupTimeouts = keywords.Group{
		ID:    "timeouts",
		Name:  "Timeouts",
		Text:  keywords.NewText(fs, "text/group/timeouts"),
		Order: 90,
	}
)
