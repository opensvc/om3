package render

import (
	"opensvc.com/opensvc/core/types"
)

var sections = [4]string{
	"threads",
	"arbitrators",
	"nodes",
	"services",
}

type (
	// Config exposes daemon status renderer tunables.
	Config struct {
		Paths []string
		Node  string
	}

	// Data holds current, previous and statistics datasets.
	Data struct {
		Current  types.DaemonStatus
		Previous types.DaemonStatus
		Stats    types.DaemonStats
	}
)

func renderDaemonStatus(data Data, c Config) string {
	return ""
}
