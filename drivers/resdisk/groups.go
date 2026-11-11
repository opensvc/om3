package resdisk

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords several disk drivers have, declared once here
// and added by each driver whose keywords name them. A group holds, in a
// driver, the keywords that driver declares with its id: the device of a
// logical volume is its name and its volume group, the one of a loop its
// file.
//
// The drivers add their own groups, as the passphrase of a crypt disk, in
// the order between these. The SCSI persistent reservation keywords every
// disk driver has are in the reservation group every resource may name, as
// the multihost protection of a zpool is.
var (
	// GroupDevice holds what names the device the resource activates, on
	// the node or on the storage.
	GroupDevice = keywords.Group{
		ID:    "device",
		Name:  "Device",
		Text:  keywords.NewText(fs, "text/group/device"),
		Order: 10,
	}

	// GroupCreation holds how the provision creates the device: its size,
	// what it is made of, and the options of the command creating it.
	GroupCreation = keywords.Group{
		ID:    "creation",
		Name:  "Creation",
		Text:  keywords.NewText(fs, "text/group/creation"),
		Order: 20,
	}

	// GroupReplication holds how the devices are replicated between the
	// nodes or the sites, and how the start takes the replica over.
	GroupReplication = keywords.Group{
		ID:    "replication",
		Name:  "Replication",
		Text:  keywords.NewText(fs, "text/group/replication"),
		Order: 30,
	}

	// GroupAccess holds how the node accesses the devices the resource
	// exposes.
	GroupAccess = keywords.Group{
		ID:    "access",
		Name:  "Access",
		Text:  keywords.NewText(fs, "text/group/access"),
		Order: 60,
	}

	// GroupTimeouts holds how long the actions on the storage may take. It
	// has the id, name and order of the timeouts of the containers and the
	// tasks.
	GroupTimeouts = keywords.Group{
		ID:    "timeouts",
		Name:  "Timeouts",
		Text:  keywords.NewText(fs, "text/group/timeouts"),
		Order: 90,
	}
)
