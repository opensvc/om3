package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-collections/collections/set"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/sizeconv"
)

func (f Frame) sNodeScoreLine() string {
	var sb strings.Builder
	sb.WriteString(format(" %s\t\t\t%s\t", bold("score"), f.info.separator))
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeScore(n))
		sb.WriteString("\t")
	}
	return sb.String()
}

func (f Frame) sNodeLoadLine() string {
	var sb strings.Builder
	sb.WriteString(format(" %s\t\t\t%s\t", bold("load15m"), f.info.separator))
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeLoad(n))
		sb.WriteString("\t")
	}
	return sb.String()
}

func (f Frame) sNodeMemLine() string {
	var sb strings.Builder
	sb.WriteString(format("  %s\t\t\t%s\t", bold("mem"), f.info.separator))
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeMem(n))
		sb.WriteString("\t")
	}
	return sb.String()
}

func (f Frame) sNodeSwapLine() string {
	var sb strings.Builder
	sb.WriteString(format(" %s\t\t\t%s\t", bold("swap"), f.info.separator))
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeSwap(n))
		sb.WriteString("\t")
	}
	return sb.String()
}

func (f Frame) StrNodeStates(n string) string {
	var sb strings.Builder
	sb.WriteString(f.sNodeMonState(n))
	sb.WriteString(f.sNodeFrozen(n))
	sb.WriteString(f.sNodeMonTarget(n))
	return sb.String()
}

func (f Frame) sNodeWarningsLine() string {
	var sb strings.Builder
	sb.WriteString(format(" %s\t\t\t%s\t", bold("state"), f.info.separator))
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeStates(n))
		sb.WriteString("\t")
	}
	return sb.String()
}

func (f Frame) sNodeVersionLine() string {
	versions := set.New()
	for _, n := range f.Current.Cluster.Config.Nodes {
		versions.Insert(f.sNodeVersion(n))
	}
	if versions.Len() == 1 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(format(" %s\t%s\t\t%s\t", bold("version"), yellow("warn"), f.info.separator))
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.sNodeVersion(n))
		sb.WriteString("\t")
	}
	return sb.String() + "\n"
}

func (f Frame) sNodeCompatLine() string {
	if f.Current.Cluster.Status.IsCompat {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(format("  %s\t%s\t\t%s\t", bold("compat"), yellow("warn"), f.info.separator))
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.sNodeCompat(n))
		sb.WriteString("\t")
	}
	return sb.String() + "\n"
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
		limit := 100 - val.Config.MinAvailMemPct
		usage := 100 - val.Stats.MemAvailPct
		total := sizeconv.BSizeCompactFromMB(val.Stats.MemTotalMB)
		var sb strings.Builder
		if val.Config.MinAvailMemPct > 0 {
			sb.WriteString(format("%d%%%s<%d%%", usage, total, limit))
		} else {
			sb.WriteString(format("%d%%%s", usage, total))
		}
		if usage > limit {
			return hired(sb.String())
		}
		return sb.String()
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
		limit := 100 - val.Config.MinAvailSwapPct
		usage := 100 - val.Stats.SwapAvailPct
		total := sizeconv.BSizeCompactFromMB(val.Stats.SwapTotalMB)
		var sb strings.Builder
		if val.Config.MinAvailSwapPct > 0 {
			sb.WriteString(format("%d%%%s<%d%%", usage, total, limit))
		} else {
			sb.WriteString(format("%d%%%s", usage, total))
		}
		if usage > limit {
			return hired(sb.String())
		}
		return sb.String()
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
		var sb strings.Builder
		switch val.Monitor.GlobalExpect {
		case node.MonitorGlobalExpectNone:
		case node.MonitorGlobalExpectInit:
		default:
			sb.WriteString(rawconfig.Colorize.Frozen(" >" + val.Monitor.GlobalExpect.String()))
		}
		switch val.Monitor.LocalExpect {
		case node.MonitorLocalExpectNone:
		case node.MonitorLocalExpectInit:
		default:
			sb.WriteString(rawconfig.Colorize.Frozen(" >" + val.Monitor.LocalExpect.String()))
		}
		return sb.String()
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
	var sb strings.Builder
	sb.WriteString(format(" %s\t\t\t%s", bold("hb-q"), f.info.separator+"\t"))
	for _, peer := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeHbMode(peer))
		sb.WriteString("\t")
	}
	return sb.String()
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

func (f Frame) sNodeUptimeLine() string {
	var sb strings.Builder
	sb.WriteString(format(" %s\t\t\t%s\t", bold("uptime"), f.info.separator))
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeUptime(n))
		sb.WriteString("\t")
	}
	return sb.String()
}

func (f Frame) StrNodeUptime(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		diffTime := time.Now().Sub(val.Status.BootedAt)
		return f.formatDuration(diffTime)
	}
	return iconUndef
}

func (f Frame) formatDuration(t time.Duration) string {
	var sb strings.Builder
	day := 24 * time.Hour
	if t < time.Hour {
		if t >= time.Minute {
			sb.WriteString(format("%dm", int(t.Minutes())))
		}
		sb.WriteString(format("%ds", int(t.Seconds())%60))
		return sb.String()
	}
	if t >= day {
		sb.WriteString(format("%dd", int(t.Hours())/24))
	}
	if t < 10*day {
		sb.WriteString(format("%dh", int(t.Hours())%24))
	}
	return sb.String()
}

func (f Frame) wNodes() {
	fmt.Fprintln(f.w, f.title("Nodes"))
	fmt.Fprintln(f.w, f.sNodeUptimeLine())
	fmt.Fprintln(f.w, f.sNodeScoreLine())
	fmt.Fprintln(f.w, f.sNodeLoadLine())
	fmt.Fprintln(f.w, f.sNodeMemLine())
	fmt.Fprintln(f.w, f.sNodeSwapLine())
	fmt.Fprint(f.w, f.sNodeVersionLine())
	fmt.Fprint(f.w, f.sNodeCompatLine())
	fmt.Fprintln(f.w, f.sNodeWarningsLine())
	fmt.Fprintln(f.w, f.info.empty)
}
