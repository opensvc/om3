package render

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	tabwriter "github.com/juju/ansiterm"
	tsize "github.com/kopoli/go-terminal-size"

	"opensvc.com/opensvc/core/converters/sizeconv"
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
	info := scanData(data)
	w := tabwriter.NewTabWriter(os.Stdout, 1, 1, 1, ' ', 0)
	wThreads(w, data, info)
	wArbitrators(w, data, info)
	wNodes(w, data, info)
	wObjects(w, data, info)
	w.Flush()
	return ""
}

type dataInfo struct {
	nodeCount   int
	arbitrators map[string]int
	empty       string
	emptyNodes  string
	separator   string
	columns     int
}

func scanData(data DaemonStatusData) *dataInfo {
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
	return info
}

func wThreadDaemon(data DaemonStatusData, info *dataInfo) string {
	var s string
	s += bold(" daemon") + "\t"
	s += green("running") + "\t"
	s += "\t"
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func wThreadCollector(data DaemonStatusData, info *dataInfo) string {
	var s string
	s += bold(" collector") + "\t"
	if data.Current.Collector.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += info.separator + "\t"
	for _, v := range data.Current.Monitor.Nodes {
		if v.Speaker {
			s += green("O") + "\t"
		} else {
			s += "\t"
		}
	}
	return s
}

func wThreadListener(data DaemonStatusData, info *dataInfo) string {
	var s string
	s += bold(" listener") + "\t"
	if data.Current.Listener.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += fmt.Sprintf("%s\t", Listener(data.Current.Listener.Config.Addr, data.Current.Listener.Config.Port))
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func wThreadScheduler(data DaemonStatusData, info *dataInfo) string {
	var s string
	s += bold(" scheduler") + "\t"
	if data.Current.Scheduler.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func wThreadMonitor(data DaemonStatusData, info *dataInfo) string {
	var s string
	s += bold(" monitor") + "\t"
	if data.Current.Monitor.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func wThreadDNS(data DaemonStatusData, info *dataInfo) string {
	var s string
	s += bold(" dns") + "\t"
	if data.Current.DNS.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func wThreadHeartbeat(name string, data types.HeartbeatThreadStatus, info *dataInfo) string {
	var s string
	s += bold(" "+name) + "\t"
	if data.State == "running" {
		s += green("running") + sThreadAlerts(data.Alerts) + "\t"
	} else {
		s += red("stopped") + sThreadAlerts(data.Alerts) + "\t"
	}
	s += "\t"
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func sThreadAlerts(data []types.ThreadAlert) string {
	if len(data) > 0 {
		return yellow("!")
	}
	return ""
}

func sNodeScoreLine(data DaemonStatusData, info *dataInfo) string {
	s := fmt.Sprintf(" %s\t\t\t%s\t", bold("score"), info.separator)
	for _, n := range data.Current.Cluster.Nodes {
		s += sNodeScore(n, data) + "\t"
	}
	return s
}
func sNodeLoadLine(data DaemonStatusData, info *dataInfo) string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("load15m"), info.separator)
	for _, n := range data.Current.Cluster.Nodes {
		s += sNodeLoad(n, data) + "\t"
	}
	return s
}

func sNodeMemLine(data DaemonStatusData, info *dataInfo) string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("mem"), info.separator)
	for _, n := range data.Current.Cluster.Nodes {
		s += sNodeMem(n, data) + "\t"
	}
	return s
}

func sNodeSwapLine(data DaemonStatusData, info *dataInfo) string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("swap"), info.separator)
	for _, n := range data.Current.Cluster.Nodes {
		s += sNodeSwap(n, data) + "\t"
	}
	return s
}

func sNodeScore(n string, data DaemonStatusData) string {
	if val, ok := data.Current.Monitor.Nodes[n]; ok {
		return fmt.Sprintf("%d", val.Stats.Score)
	}
	return ""
}

func sNodeLoad(n string, data DaemonStatusData) string {
	if val, ok := data.Current.Monitor.Nodes[n]; ok {
		return fmt.Sprintf("%.1f", val.Stats.Load15M)
	}
	return ""
}

func sNodeMem(n string, data DaemonStatusData) string {
	if val, ok := data.Current.Monitor.Nodes[n]; ok {
		if val.Stats.MemTotalMB == 0 {
			return hiblue("-")
		}
		if val.Stats.MemAvailPct == 0 {
			return hiblue("-")
		}
		limit := 100 - val.MinAvailMemPct
		usage := 100 - val.Stats.MemAvailPct
		total := sizeconv.BSizeCompactFromMB(val.Stats.MemTotalMB)
		var s string
		if limit > 0 {
			s = fmt.Sprintf("%d/%d%%:%s", usage, limit, total)
		} else {
			s = fmt.Sprintf("%d%%:%s", usage, total)
		}
		if usage > limit {
			return red(s)
		}
		return s
	}
	return ""
}

func sNodeSwap(n string, data DaemonStatusData) string {
	if val, ok := data.Current.Monitor.Nodes[n]; ok {
		if val.Stats.SwapTotalMB == 0 {
			return hiblue("-")
		}
		if val.Stats.SwapAvailPct == 0 {
			return hiblue("-")
		}
		limit := 100 - val.MinAvailSwapPct
		usage := 100 - val.Stats.SwapAvailPct
		total := sizeconv.BSizeCompactFromMB(val.Stats.SwapTotalMB)
		var s string
		if limit > 0 {
			s = fmt.Sprintf("%d/%d%%:%s", usage, limit, total)
		} else {
			s = fmt.Sprintf("%d%%:%s", usage, total)
		}
		if usage > limit {
			return red(s)
		}
		return s
	}
	return ""
}

func wThreads(w io.Writer, data DaemonStatusData, info *dataInfo) {
	fmt.Fprintln(w, title("Threads", data))
	fmt.Fprintln(w, wThreadDaemon(data, info))
	fmt.Fprintln(w, wThreadDNS(data, info))
	fmt.Fprintln(w, wThreadCollector(data, info))
	for k, v := range data.Current.Heartbeats {
		fmt.Fprintln(w, wThreadHeartbeat(k, v, info))
	}
	fmt.Fprintln(w, wThreadListener(data, info))
	fmt.Fprintln(w, wThreadMonitor(data, info))
	fmt.Fprintln(w, wThreadScheduler(data, info))
	fmt.Fprintln(w, info.empty)
}

func wArbitrators(w io.Writer, data DaemonStatusData, info *dataInfo) {
	if len(info.arbitrators) == 0 {
		return
	}
	fmt.Fprintln(w, title("Arbitrators", data))
	fmt.Fprintln(w, info.empty)
}

func wNodes(w io.Writer, data DaemonStatusData, info *dataInfo) {
	fmt.Fprintln(w, title("Nodes", data))
	fmt.Fprintln(w, sNodeScoreLine(data, info))
	fmt.Fprintln(w, sNodeLoadLine(data, info))
	fmt.Fprintln(w, sNodeMemLine(data, info))
	fmt.Fprintln(w, sNodeSwapLine(data, info))
	fmt.Fprintln(w, info.empty)
}

func wObjects(w io.Writer, data DaemonStatusData, info *dataInfo) {
	fmt.Fprintln(w, title("Objects", data))
}

func title(s string, data DaemonStatusData) string {
	s += "\t\t\t\t"
	for _, v := range data.Current.Cluster.Nodes {
		s += bold(v) + "\t"
	}
	return s
}
