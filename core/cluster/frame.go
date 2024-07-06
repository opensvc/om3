package cluster

import (
	"sort"
	"strings"

	"github.com/fatih/color"
	tabwriter "github.com/juju/ansiterm"
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

	green   = color.New(color.FgGreen).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	red     = color.New(color.FgRed).SprintFunc()
	blue    = color.New(color.FgBlue).SprintFunc()
	hiblue  = color.New(color.FgHiBlue).SprintFunc()
	hiblack = color.New(color.FgHiBlack).SprintFunc()
	bold    = color.New(color.Bold).SprintFunc()

	iconUp             = green("O")
	iconWarning        = yellow("!")
	iconDownIssue      = red("X")
	iconPlacementAlert = red("^")
	iconProvisionAlert = red("P")
	iconStandbyDown    = red("x")
	iconStandbyUpIssue = red("o")
	iconUndef          = red("?")
	iconFrozen         = blue("*")
	iconDown           = hiblack("X")
	iconDRP            = hiblack("#")
	iconLeader         = hiblack("^")
	iconNotApplicable  = hiblack("/")
	iconPreserved      = hiblack("?")
	iconStandbyUp      = hiblack("o")
)

type (
	// Frame exposes daemon status renderer tunables.
	Frame struct {
		Selector string
		Nodes    []string
		Sections []string
		Current  Data
		Previous Data
		Stats    Stats
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
	f.setSectionMask()
	f.scanData()
	f.w = tabwriter.NewTabWriter(&builder, 1, 1, 1, ' ', 0)
	if f.hasSection("threads") {
		f.wThreads()
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
