package resapp

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords of the app drivers, app.forking and app.simple,
// declared once here and added by each app driver. They have the ids, names
// and orders of the groups of the containers and tasks holding the same
// topics, so a reader sees one vocabulary.
//
// The host task borrows the keywords of the app drivers, which name these
// groups by their ids: it adds the environment and limits groups, and its
// own process group, its command running on the node or in a container.
//
// The script running every command an app keyword sets to true is no
// strategy of the process group: a command set to true runs the script, a
// command set to a shell expression runs it, and an app mixes both.
var (
	// GroupIdentity holds the label the resource is shown with.
	GroupIdentity = keywords.Group{
		ID:    "identity",
		Name:  "Identity",
		Text:  keywords.NewText(fs, "text/group/identity"),
		Order: 10,
	}

	// GroupProcess holds the commands the app runs, and the identity,
	// directory and network namespace they run with.
	GroupProcess = keywords.Group{
		ID:    "process",
		Name:  "Process",
		Text:  keywords.NewText(fs, "text/group/process"),
		Order: 30,
	}

	// GroupStatus holds how the check of the app makes the status of its
	// resource. It has the id, name and order of the status of the tasks.
	GroupStatus = keywords.Group{
		ID:    "status",
		Name:  "Status",
		Text:  keywords.NewText(fs, "text/group/status"),
		Order: 36,
	}

	// GroupEnvironment holds the variables the commands are given.
	GroupEnvironment = keywords.Group{
		ID:    "environment",
		Name:  "Environment",
		Text:  keywords.NewText(fs, "text/group/environment"),
		Order: 40,
	}

	// GroupLimits holds the limits om exec sets on a command of the node
	// before it runs it.
	GroupLimits = keywords.Group{
		ID:    "limits",
		Name:  "Limits",
		Text:  keywords.NewText(fs, "text/group/limits"),
		Order: 44,
	}

	// GroupQuiesce holds the commands suspending and resuming the writes
	// of the app for a run of a sync resource. It has the id, name and
	// order of the quiesce of the oci containers, with none of their
	// strategies: an app quiesces by a command alone.
	GroupQuiesce = keywords.Group{
		ID:    "quiesce",
		Name:  "Quiesce",
		Text:  keywords.NewText(fs, "text/group/quiesce"),
		Order: 75,
	}

	// GroupTimeouts holds how long the commands of the app may take. It
	// has the id, name and order of the timeouts of the containers and
	// tasks.
	GroupTimeouts = keywords.Group{
		ID:    "timeouts",
		Name:  "Timeouts",
		Text:  keywords.NewText(fs, "text/group/timeouts"),
		Order: 90,
	}

	// Groups are the groups the keywords of an app driver name.
	Groups = []keywords.Group{
		GroupIdentity,
		GroupProcess,
		GroupStatus,
		GroupEnvironment,
		GroupLimits,
		GroupQuiesce,
		GroupTimeouts,
	}
)
