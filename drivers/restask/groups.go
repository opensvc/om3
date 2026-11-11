package restask

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords every task driver has, declared once here and
// added by each task driver. The drivers add their own groups, as the image
// of an oci task or the certificates of an acme task, in the order between
// these.
var (
	// GroupScheduling holds when the task runs: by its schedule, or on
	// the events of the node, at a pace.
	GroupScheduling = keywords.Group{
		ID:    "scheduling",
		Name:  "Scheduling",
		Text:  keywords.NewText(fs, "text/group/scheduling"),
		Order: 32,
	}

	// GroupRun holds how a run of the task goes: the confirmation it
	// asks, how many run at once, its outputs logged, and the command run
	// when it fails.
	GroupRun = keywords.Group{
		ID:    "run",
		Name:  "Runs",
		Text:  keywords.NewText(fs, "text/group/run"),
		Order: 34,
	}

	// GroupStatus holds how the runs of the task make the status of its
	// resource.
	GroupStatus = keywords.Group{
		ID:    "status",
		Name:  "Status",
		Text:  keywords.NewText(fs, "text/group/status"),
		Order: 36,
	}

	// GroupInstall holds the volumes and filesystems whose files the task
	// installs again before its own work.
	GroupInstall = keywords.Group{
		ID:    "install",
		Name:  "Install",
		Text:  keywords.NewText(fs, "text/group/install"),
		Order: 46,
	}

	// GroupTimeouts holds how long a run of the task may take, and how
	// long its stop waits for the runs to end. It has the id, name and
	// order of the timeouts of the containers.
	GroupTimeouts = keywords.Group{
		ID:    "timeouts",
		Name:  "Timeouts",
		Text:  keywords.NewText(fs, "text/group/timeouts"),
		Order: 90,
	}

	// Groups are the groups the keywords every task driver has name.
	Groups = []keywords.Group{
		GroupScheduling,
		GroupRun,
		GroupStatus,
		GroupInstall,
		GroupTimeouts,
	}
)
