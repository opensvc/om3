package monitor

import (
	"sort"
	"strings"

	"github.com/fatih/color"
	tabwriter "github.com/juju/ansiterm"

	"github.com/opensvc/om3/core/clusterdump"
)

const (
	staticCols = 3

	sectionThreads int = 1 << iota
	sectionArbitrators
	sectionNodes
	sectionObjects
)

var (
	sectionToID = map[string]int{
		"threads":     sectionThreads,
		"arbitrators": sectionArbitrators,
		"nodes":       sectionNodes,
		"objects":     sectionObjects,
		"services":    sectionObjects,
	}
	green, yellow, hired, red, blue, hiblue, hiblack, bold                                                                                                                                                                 func(a ...interface{}) string
	iconUp, iconWarning, iconDownIssue, iconPlacementAlert, iconProvisionAlert, iconStandbyDown, iconStandbyUpIssue, iconUndef, iconFrozen, iconDown, iconDRP, iconLeader, iconNotApplicable, iconPreserved, iconStandbyUp string
)

func InitColor() {
	green = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	red = color.New(color.FgRed).SprintFunc()
	hired = color.New(color.FgHiRed).SprintFunc()
	blue = color.New(color.FgBlue).SprintFunc()
	hiblue = color.New(color.FgHiBlue).SprintFunc()
	hiblack = color.New(color.FgHiBlack).SprintFunc()
	bold = color.New(color.Bold).SprintFunc()

	iconUp = green("O")
	iconWarning = yellow("!")
	iconDownIssue = hired("X")
	iconPlacementAlert = hired("^")
	iconProvisionAlert = hired("P")
	iconStandbyDown = hired("x")
	iconStandbyUpIssue = hired("o")
	iconUndef = hired("?")
	iconFrozen = bold(hiblue("*"))
	iconDown = hiblack("X")
	iconDRP = hiblack("#")
	iconLeader = hiblack("^")
	iconNotApplicable = hiblack("/")
	iconPreserved = hiblack("?")
	iconStandbyUp = hiblack("o")
}

type (
	// Frame exposes daemon status renderer tunables.
	Frame struct {
		Selector string
		Nodes    []string
		Sections []string
		Current  clusterdump.Data
		Previous clusterdump.Data
		Stats    clusterdump.Stats
		// Nodename is the nodename from which we have received data (value of
		// .daemon.nodename)
		Nodename string

		// private
		w           *tabwriter.TabWriter
		sectionMask int
		info        struct {
			nodeCount   int
			arbitrators map[string]int
			empty       string
			emptyNodes  string
			separator   string
			columns     int
			paths       []string
		}
	}
)

func (f *Frame) setSectionMask() {
	i := 0
	for _, s := range f.Sections {
		i += sectionToID[s]
	}
	f.sectionMask = i
}

func (f Frame) hasSection(section string) bool {
	if f.sectionMask == 0 {
		return true
	}
	return f.sectionMask&sectionToID[section] != 0
}

// Render return a string buffer containing a human-friendly
// representation of Render.
func (f *Frame) Render() string {
	var builder strings.Builder
	InitColor()

	green = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	red = color.New(color.FgRed).SprintFunc()
	hired = color.New(color.FgHiRed).SprintFunc()
	blue = color.New(color.FgBlue).SprintFunc()
	hiblue = color.New(color.FgHiBlue).SprintFunc()
	hiblack = color.New(color.FgHiBlack).SprintFunc()
	bold = color.New(color.Bold).SprintFunc()

	f.setSectionMask()
	f.scanData()
	f.w = tabwriter.NewTabWriter(&builder, 1, 1, 1, ' ', 0)
	if f.hasSection("daemons") {
		f.wDaemons()
	}
	if f.hasSection("arbitrators") {
		f.wArbitrators()
	}
	if f.hasSection("nodes") {
		f.wNodes()
	}
	if f.hasSection("objects") {
		f.wObjects()
	}
	f.w.Flush()
	return builder.String()
}

func (f *Frame) scanData() {
	f.info.nodeCount = len(f.Current.Cluster.Config.Nodes)
	// +1 for the separator between static cols and node cols
	f.info.columns = staticCols + f.info.nodeCount + 1
	f.info.empty = strings.Repeat("\t", f.info.columns)
	f.info.emptyNodes = strings.Repeat("\t", f.info.nodeCount)
	if f.info.nodeCount > 0 {
		f.info.separator = "|"
	} else {
		f.info.separator = " "
	}
	f.info.arbitrators = make(map[string]int)
	for _, v := range f.Current.Cluster.Node {
		for name := range v.Status.Arbitrators {
			f.info.arbitrators[name] = 1
		}
	}
	f.info.paths = make([]string, 0)
	for path := range f.Current.Cluster.Object {
		f.info.paths = append(f.info.paths, path)
	}
	sort.Strings(f.info.paths)
}

func (f Frame) title(s string) string {
	s += "\t\t\t\t"
	for _, v := range f.Current.Cluster.Config.Nodes {
		s += bold(v) + "\t"
	}
	return s
}
