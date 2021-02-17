package render

import (
	"fmt"
	"io"
	"os"
	"strings"

	tabwriter "github.com/juju/ansiterm"

	"github.com/fatih/color"
	tsize "github.com/kopoli/go-terminal-size"

	"opensvc.com/opensvc/core/types"
)

var (
	sections = [4]string{
		"threads",
		"arbitrators",
		"nodes",
		"services",
	}
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	blue   = color.New(color.FgBlue).SprintFunc()
	hiblue = color.New(color.FgHiBlue).SprintFunc()
	bold   = color.New(color.Bold).SprintFunc()
)

const (
	staticCols = 3
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

// GetOutputTermSize returns the stdout terminal size or defaults
func GetOutputTermSize() tsize.Size {
	ts, err := tsize.FgetSize(os.Stdout)
	if err != nil {
		return tsize.Size{Height: 250, Width: 800}
	}
	return ts
}

// DaemonStatus return a string buffer containing a human-friendly
// representation of DaemonStatus.
func DaemonStatus(data DaemonStatusData, c DaemonStatusOptions) string {
	//ts := GetOutputTermSize()
	cols := nCols(data)
	empty := sEmpty(data, cols)
	w := tabwriter.NewTabWriter(os.Stdout, 1, 1, 1, ' ', 0)
	wThreads(w, data)
	fmt.Fprintln(w, empty)

	wArbitrators(w, data)
	fmt.Fprintln(w, empty)

	wNodes(w, data)
	fmt.Fprintln(w, empty)

	wObjects(w, data)
	w.Flush()
	return ""
}

func nCols(data DaemonStatusData) int {
	return staticCols + len(data.Current.Cluster.Nodes)
}

func sEmpty(data DaemonStatusData, cols int) string {
	return strings.Repeat(" \t", cols+1)
}

func wThreadDaemon(data DaemonStatusData) string {
	var s string
	s += bold(" daemon") + "\t"
	s += green("running") + "\t"
	s += "\t"
	s += "|\t"
	for _, v := range data.Current.Monitor.Nodes {
		if v.Speaker {
			s += green("O") + "\t"
		} else {
			s += "\t"
		}
	}
	return s
}
func wThreads(w io.Writer, data DaemonStatusData) {
	fmt.Fprintln(w, title("Threads", data))
	fmt.Fprintln(w, wThreadDaemon(data))
}

func wArbitrators(w io.Writer, data DaemonStatusData) {
	fmt.Fprintln(w, title("Arbitrators", data))
}

func wNodes(w io.Writer, data DaemonStatusData) {
	fmt.Fprintln(w, title("Nodes", data))
}

func wObjects(w io.Writer, data DaemonStatusData) {
	fmt.Fprintln(w, title("Objects", data))
}

func title(s string, data DaemonStatusData) string {
	s += "\t\t\t\t"
	for _, v := range data.Current.Cluster.Nodes {
		s += bold(v) + "\t"
	}
	return s
}
