package ressync

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords several sync drivers have, declared once here
// and added by each driver whose keywords name them. A group holds, in a
// driver, the keywords that driver declares with its id: the source of an
// rsync is a path, the one of a zfs sync a dataset.
//
// The scheduling, status and timeouts groups have the ids, names and orders
// of the ones of the task drivers, their texts saying the updates of a sync.
var (
	// GroupEndpoints holds what the resource copies, and where to: the
	// source, the destination, and the nodes the copies go to.
	GroupEndpoints = keywords.Group{
		ID:    "endpoints",
		Name:  "Source and target",
		Text:  keywords.NewText(fs, "text/group/endpoints"),
		Order: 10,
	}

	// GroupArray holds the storage array, and its devices or groups, the
	// resource drives the replication or the snapshots of.
	GroupArray = keywords.Group{
		ID:    "array",
		Name:  "Array",
		Text:  keywords.NewText(fs, "text/group/array"),
		Order: 12,
	}

	// GroupSnapshots holds what the resource snapshots, and how.
	GroupSnapshots = keywords.Group{
		ID:    "snapshots",
		Name:  "Snapshots",
		Text:  keywords.NewText(fs, "text/group/snapshots"),
		Order: 20,
	}

	// GroupTransfer holds how an update copies the data.
	GroupTransfer = keywords.Group{
		ID:    "transfer",
		Name:  "Transfer",
		Text:  keywords.NewText(fs, "text/group/transfer"),
		Order: 24,
	}

	// GroupScheduling holds when the resource updates on its own, and
	// whether these updates are logged.
	GroupScheduling = keywords.Group{
		ID:    "scheduling",
		Name:  "Scheduling",
		Text:  keywords.NewText(fs, "text/group/scheduling"),
		Order: 32,
	}

	// GroupStatus holds how old a copy may be before the status of the
	// resource warns.
	GroupStatus = keywords.Group{
		ID:    "status",
		Name:  "Status",
		Text:  keywords.NewText(fs, "text/group/status"),
		Order: 36,
	}

	// GroupRetention holds what the resource keeps of what it made: the
	// snapshots and runs, and the base snapshots kept for a peer missing
	// updates.
	GroupRetention = keywords.Group{
		ID:    "retention",
		Name:  "Retention",
		Text:  keywords.NewText(fs, "text/group/retention"),
		Order: 40,
	}

	// GroupTimeouts holds how long an update may take.
	GroupTimeouts = keywords.Group{
		ID:    "timeouts",
		Name:  "Timeouts",
		Text:  keywords.NewText(fs, "text/group/timeouts"),
		Order: 90,
	}
)
