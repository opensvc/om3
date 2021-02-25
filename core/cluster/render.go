package cluster

import (
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	tabwriter "github.com/juju/ansiterm"
	tsize "github.com/kopoli/go-terminal-size"
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
	iconDown           = red("X")
	iconPlacementAlert = red("^")
	iconProvisionAlert = red("P")
	iconStandbyDown    = red("o")
	iconStandbyUpIssue = red("x")
	iconUndef          = red("?")
	iconFrozen         = blue("*")
	iconDRP            = hiblack("#")
	iconLeader         = hiblack("^")
	iconNotApplicable  = hiblack("/")
	iconPreserved      = hiblack("?")
	iconStandbyUp      = hiblack("x")
)

type (
	// Options exposes daemon status renderer tunables.
	Options struct {
		Nodes    []string
		Sections []string
	}

	// Data holds current, previous and statistics datasets.
	Data struct {
		Current  Status
		Previous Status
		Stats    Stats
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

func sectionMask(sections []string) int {
	i := 0
	for _, s := range sections {
		i += sectionToID[s]
	}
	return i
}

func hasSection(mask int, section string) bool {
	if mask == 0 {
		return true
	}
	return mask&sectionToID[section] != 0
}

// Render return a string buffer containing a human-friendly
// representation of Render.
func Render(data Data, opts Options) string {
	//ts := GetOutputTermSize()
	var builder strings.Builder
	sm := sectionMask(opts.Sections)
	info := scanData(data)
	w := tabwriter.NewTabWriter(&builder, 1, 1, 1, ' ', 0)
	if hasSection(sm, "threads") {
		wThreads(w, data, info)
	}
	if hasSection(sm, "arbitrators") {
		wArbitrators(w, data, info)
	}
	if hasSection(sm, "nodes") {
		wNodes(w, data, info)
	}
	if hasSection(sm, "objects") {
		wObjects(w, data, info)
	}
	w.Flush()
	return builder.String()
}

type dataInfo struct {
	nodeCount   int
	arbitrators map[string]int
	empty       string
	emptyNodes  string
	separator   string
	columns     int
	paths       []string
}

func scanData(data Data) *dataInfo {
	info := &dataInfo{}
	info.nodeCount = len(data.Current.Cluster.Nodes)
	// +1 for the separator between static cols and node cols
	info.columns = staticCols + info.nodeCount + 1
	info.empty = strings.Repeat("\t", info.columns)
	info.emptyNodes = strings.Repeat("\t", info.nodeCount)
	if info.nodeCount > 0 {
		info.separator = "|"
	} else {
		info.separator = " "
	}
	for _, v := range data.Current.Monitor.Nodes {
		for name := range v.Arbitrators {
			info.arbitrators[name] = 1
		}
	}
	info.paths = make([]string, 0)
	for path := range data.Current.Monitor.Services {
		info.paths = append(info.paths, path)
	}
	sort.Strings(info.paths)
	return info
}

func title(s string, data Data) string {
	s += "\t\t\t\t"
	for _, v := range data.Current.Cluster.Nodes {
		s += bold(v) + "\t"
	}
	return s
}
