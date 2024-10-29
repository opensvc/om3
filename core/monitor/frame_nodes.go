package monitor

import (
	"fmt"

	"github.com/golang-collections/collections/set"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/sizeconv"
)

func (f Frame) sNodeScoreLine() string {
	s := fmt.Sprintf(" %s\t\t\t%s\t", bold("score"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.StrNodeScore(n) + "\t"
	}
	return s
}
func (f Frame) sNodeLoadLine() string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("load15m"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.StrNodeLoad(n) + "\t"
	}
	return s
}

func (f Frame) sNodeMemLine() string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("mem"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.StrNodeMem(n) + "\t"
	}
	return s
}

func (f Frame) sNodeSwapLine() string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("swap"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.StrNodeSwap(n) + "\t"
	}
	return s
}

func (f Frame) StrNodeStates(n string) string {
	s := f.sNodeMonState(n)
	s += f.sNodeFrozen(n)
	s += f.sNodeMonTarget(n)
	return s
}

func (f Frame) sNodeWarningsLine() string {
	s := fmt.Sprintf(" %s\t\t\t%s\t", bold("state"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.StrNodeStates(n) + "\t"
	}
	return s
}

func (f Frame) sNodeVersionLine() string {
	versions := set.New()
	for _, n := range f.Current.Cluster.Config.Nodes {
		versions.Insert(f.sNodeVersion(n))
	}
	if versions.Len() == 1 {
		return ""
	}
	s := fmt.Sprintf("  %s\t%s\t\t%s\t", bold("version"), yellow("warn"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.sNodeVersion(n) + "\t"
	}
	return s + "\n"
}

func (f Frame) sNodeCompatLine() string {
	if f.Current.Cluster.Status.IsCompat {
		return ""
	}
	s := fmt.Sprintf("  %s\t%s\t\t%s\t", bold("compat"), yellow("warn"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.sNodeCompat(n) + "\t"
	}
	return s + "\n"
}

func (f Frame) StrNodeScore(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		return fmt.Sprintf("%d", val.Stats.Score)
	}
	return iconUndef
}

func (f Frame) StrNodeLoad(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		return fmt.Sprintf("%.1f", val.Stats.Load15M)
	}
	return iconUndef
}

func (f Frame) StrNodeMem(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		if val.Stats.MemTotalMB == 0 {
			return hiblue("-")
		}
		if val.Stats.MemAvailPct == 0 {
			return hiblue("-")
		}
		limit := 100 - val.Status.MinAvailMemPct
		usage := 100 - val.Stats.MemAvailPct
		total := sizeconv.BSizeCompactFromMB(val.Stats.MemTotalMB)
		var s string
		if limit > 0 {
			s = fmt.Sprintf("%d%%%s<%d%%", usage, total, limit)
		} else {
			s = fmt.Sprintf("%d%%%s", usage, total)
		}
		if usage > limit {
			return hired(s)
		}
		return s
	}
	return iconUndef
}

func (f Frame) StrNodeSwap(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		if val.Stats.SwapTotalMB == 0 {
			return hiblue("-")
		}
		if val.Stats.SwapAvailPct == 0 {
			return hiblue("-")
		}
		limit := 100 - val.Status.MinAvailSwapPct
		usage := 100 - val.Stats.SwapAvailPct
		total := sizeconv.BSizeCompactFromMB(val.Stats.SwapTotalMB)
		var s string
		if limit > 0 {
			s = fmt.Sprintf("%d%%%s<%d%%", usage, total, limit)
		} else {
			s = fmt.Sprintf("%d%%%s", usage, total)
		}
		if usage > limit {
			return hired(s)
		}
		return s
	}
	return iconUndef
}

func (f Frame) sNodeMonState(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		if val.Monitor.State != node.MonitorStateIdle {
			return val.Monitor.State.String()
		}
	}
	return ""
}

func (f Frame) sNodeFrozen(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		if !val.Status.FrozenAt.IsZero() {
			return iconFrozen
		}
	}
	return ""
}

func (f Frame) sNodeMonTarget(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		s := ""
		switch val.Monitor.GlobalExpect {
		case node.MonitorGlobalExpectNone:
		case node.MonitorGlobalExpectInit:
		default:
			s += rawconfig.Colorize.Frozen(" >" + val.Monitor.GlobalExpect.String())
		}
		switch val.Monitor.LocalExpect {
		case node.MonitorLocalExpectNone:
		case node.MonitorLocalExpectInit:
		default:
			s += rawconfig.Colorize.Frozen(" >" + val.Monitor.LocalExpect.String())
		}
		return s
	}
	return ""
}

func (f Frame) sNodeCompat(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		return fmt.Sprintf("%d", val.Status.Compat)
	}
	return iconUndef
}

func (f Frame) sNodeVersion(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		if len(val.Status.Agent) == 40 {
			// commit id => abbrev
			return fmt.Sprintf("%s", val.Status.Agent[:8])
		} else {
			return fmt.Sprintf("%s", val.Status.Agent)
		}
	}
	return iconUndef
}

func (f Frame) sNodeHbMode() string {
	s := fmt.Sprintf(" %s\t\t\t%s", bold("hb-q"), f.info.separator+"\t")
	for _, peer := range f.Current.Cluster.Config.Nodes {
		s += f.StrNodeHbMode(peer) + "\t"
	}
	return s
}

func (f Frame) StrNodeHbMode(peer string) string {
	var mode string
	nodeCount := len(f.Current.Cluster.Config.Nodes)
	lastMessage := f.Current.Cluster.Node[peer].Daemon.Heartbeat.LastMessage
	switch lastMessage.Type {
	case "patch":
		mode = fmt.Sprintf("%d", lastMessage.PatchLength)
	default:
		mode = lastMessage.Type
	}
	switch mode {
	case "full":
		mode = yellow(mode)
	case "ping":
		if nodeCount > 1 {
			mode = yellow(mode)
		}
	case "":
		if nodeCount > 1 {
			mode = hired("?")
		} else {
			mode = "?"
		}
	}
	return mode
}

func (f Frame) wNodes() {
	fmt.Fprintln(f.w, f.title("Nodes"))
	fmt.Fprintln(f.w, f.sNodeScoreLine())
	fmt.Fprintln(f.w, f.sNodeLoadLine())
	fmt.Fprintln(f.w, f.sNodeMemLine())
	fmt.Fprintln(f.w, f.sNodeSwapLine())
	fmt.Fprint(f.w, f.sNodeVersionLine())
	fmt.Fprint(f.w, f.sNodeCompatLine())
	if len(f.Current.Cluster.Config.Nodes) > 1 {
		fmt.Fprintln(f.w, f.sNodeHbMode())
	}
	fmt.Fprintln(f.w, f.sNodeWarningsLine())
	fmt.Fprintln(f.w, f.info.empty)
}
