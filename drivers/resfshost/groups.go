package resfshost

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords of the filesystems mounted, declared once here
// and added by the drivers sharing these keywords, the zfs and sgcp_nfs ones
// included. The owner and the permissions of the root, and the files
// installed, are the groups of the datarecv keywords.
//
// No group has strategies. The tmpfs size and mode keywords override the
// size= and mode= options of mnt_opt, which are then ignored, and the btrfs
// subvol keyword refuses a subvol= option naming another subvolume only: the
// alternatives are values of mnt_opt, not keywords a strategy sets.
var (
	// GroupDevice holds what the filesystem is mounted from, and how its
	// devices are made writable at start.
	GroupDevice = keywords.Group{
		ID:    "device",
		Name:  "Device",
		Text:  keywords.NewText(fs, "text/group/device"),
		Order: 10,
	}

	// GroupMount holds where the filesystem is mounted, and with which
	// options.
	GroupMount = keywords.Group{
		ID:    "mount",
		Name:  "Mount",
		Text:  keywords.NewText(fs, "text/group/mount"),
		Order: 20,
	}

	// GroupCreation holds how the provision makes the filesystem. It has
	// the id and name of the creation group of the lxc containers.
	GroupCreation = keywords.Group{
		ID:    "creation",
		Name:  "Creation",
		Text:  keywords.NewText(fs, "text/group/creation"),
		Order: 24,
	}

	// GroupSize holds the space the filesystem holds: the memory a tmpfs
	// may use, the size of a zfs dataset and the quotas and reservations
	// derived from it.
	GroupSize = keywords.Group{
		ID:    "size",
		Name:  "Size",
		Text:  keywords.NewText(fs, "text/group/size"),
		Order: 28,
	}

	// GroupStatus holds how the status of the resource checks the mounted
	// filesystem answers. It has the id and name of the status group of
	// the tasks.
	GroupStatus = keywords.Group{
		ID:    "status",
		Name:  "Status",
		Text:  keywords.NewText(fs, "text/group/status"),
		Order: 36,
	}

	// Groups are the groups the keywords of every filesystem mounted name.
	Groups = []keywords.Group{
		GroupDevice,
		GroupMount,
		GroupCreation,
		GroupStatus,
	}
)
