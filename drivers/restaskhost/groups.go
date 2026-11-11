package restaskhost

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// GroupProcess holds the command the task runs, where it runs it, and the
// identity and directory a command of the node runs with.
//
// It has the id, name and order of the process group of the app drivers,
// whose umask, cwd, user and group keywords a host task borrows and which
// name the group by its id, but a text of its own: the command of a task
// runs on the node or in a container resource, which the commands of an app
// never do, and the task has a stop command but no start, check or info.
// The environment and limits a host task borrows from the app drivers are
// the same topics, and the task adds the declarations of the app drivers.
//
// Running the command in a container is no strategy of the process group:
// the cwd, user, group and limits a command of the node runs with apply to
// the on_error and stop commands, which run on the node either way.
var GroupProcess = keywords.Group{
	ID:    "process",
	Name:  "Process",
	Text:  keywords.NewText(fs, "text/group/process"),
	Order: 30,
}
