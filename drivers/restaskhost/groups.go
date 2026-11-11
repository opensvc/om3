package restaskhost

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords of a host task beside the ones every task driver
// has. They have the ids, names and orders of the process and environment
// groups of the oci containers and tasks, their texts saying a command of the
// node rather than the process of a container.
//
// The keywords a host task shares with the app drivers are grouped by the
// manifest, the app drivers grouping none of their keywords yet.
//
// Running the command in a container is no strategy of the process group:
// the cwd, user, group and limits a command of the node runs with apply to
// the on_error and stop commands, which run on the node either way.
var (
	// GroupProcess holds the command the task runs, where it runs it, and
	// the identity and directory a command of the node runs with.
	GroupProcess = keywords.Group{
		ID:    "process",
		Name:  "Process",
		Text:  keywords.NewText(fs, "text/group/process"),
		Order: 30,
	}

	// GroupEnvironment holds the variables the command of the task is
	// given.
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
)
