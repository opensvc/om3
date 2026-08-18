package monitor

import (
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/util/tabwriter"
)

const (
	staticCols = 3

	sectionDaemon int = 1 << iota
	sectionArbitrators
	sectionNodes
	sectionObjects
)

var (
	sectionToID = map[string]int{
		"daemon":      sectionDaemon,
		"arbitrators": sectionArbitrators,
		"nodes":       sectionNodes,
		"objects":     sectionObjects,

		// legacy sections: services and threads
		"services": sectionObjects,
		"threads":  sectionDaemon,
	}

	green, yellow, hired, hiBlue, hiBlack, bold func(a ...interface{}) string

	iconUp, iconWarning, iconDownIssue, iconPlacementAlert  string
	iconProvisionAlert, iconStandbyDown, iconStandbyUpIssue string
	iconUndef, iconFrozen, iconDown, iconDRP, iconLeader    string
	iconNotApplicable, iconPreserved, iconStandbyUp         string

	now = time.Now
)

func InitColor() {
	green = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	hired = color.New(color.FgHiRed).SprintFunc()
	hiBlue = color.New(color.FgHiBlue).SprintFunc()
	hiBlack = color.New(color.FgHiBlack).SprintFunc()
	bold = color.New(color.Bold).SprintFunc()

	iconUp = green("O")
	iconWarning = yellow("!")
	iconDownIssue = hired("X")
	iconPlacementAlert = yellow("^")
	iconProvisionAlert = hired("P")
	iconStandbyDown = hired("x")
	iconStandbyUpIssue = hired("o")
	iconUndef = hired("?")
	iconFrozen = bold(hiBlue("*"))
	iconDown = hiBlack("X")
	iconDRP = hiBlack("#")
	iconLeader = hiBlack("^")
	iconNotApplicable = hiBlack("/")
	iconPreserved = hiBlack("?")
	iconStandbyUp = hiBlack("o")
}

type (
	// Frame exposes the daemon status renderer.
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
		w           *tabwriter.Writer
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

// Render returns a string buffer containing a human-friendly
// representation of Render.
func (f *Frame) Render() string {
	var builder strings.Builder
	InitColor()

	f.setSectionMask()
	f.scanData()
	f.w = tabwriter.NewWriter(&builder, 1, 1, 1, ' ', 0)
	if f.hasSection("daemon") {
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
	_ = f.w.Flush()
	return builder.String()
}

func (f *Frame) scanData() {
	f.info.nodeCount = len(f.Current.Cluster.Config.Nodes)
	f.info.columns = staticCols + f.info.nodeCount
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
	s += "\t\t\t"
	for _, v := range f.Current.Cluster.Config.Nodes {
		s += "\t" + bold(v)
	}
	return s
}
