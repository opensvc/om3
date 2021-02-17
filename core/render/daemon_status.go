package render

import (
	"github.com/fatih/color"

	"opensvc.com/opensvc/core/types"
)

var (
	sections = [4]string{
		"threads",
		"arbitrators",
		"nodes",
		"services",
	}
	yellow = color.New(color.FgYellow).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
)

type (
	// DaemonStatusOptions exposes daemon status renderer tunables.
	DaemonStatusOptions struct {
		Paths []string
		Node  string
	}

	// DaemonStatusData holds current, previous and statistics datasets.
	DaemonStatusData struct {
		Current  types.DaemonStatus
		Previous types.DaemonStatus
		Stats    types.DaemonStats
	}
)

// DaemonStatus return a string buffer containing a human-friendly
// representation of DaemonStatus.
func DaemonStatus(data DaemonStatusData, c DaemonStatusOptions) string {
	color.NoColor = false
	return string(yellow("foo"))
}
