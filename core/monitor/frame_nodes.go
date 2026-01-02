package monitor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/golang-collections/collections/set"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/duration"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

func (f Frame) sNodeScoreLine() string {
	var sb strings.Builder
	sb.WriteString(" ")
	sb.WriteString(bold("score"))
	sb.WriteString("\t\t\t")
	sb.WriteString(f.info.separator)
	sb.WriteString("\t")
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeScore(n))
		sb.WriteString("\t")
	}
	return sb.String()
}

func (f Frame) sNodeLoadLine() string {
	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(bold("load15m"))
	sb.WriteString("\t\t\t")
	sb.WriteString(f.info.separator)
	sb.WriteString("\t")
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeLoad(n))
		sb.WriteString("\t")
	}
	return sb.String()
}

func (f Frame) sNodeMemLine() string {
	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(bold("mem"))
	sb.WriteString("\t\t\t")
	sb.WriteString(f.info.separator)
	sb.WriteString("\t")
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeMem(n))
		sb.WriteString("\t")
	}
	return sb.String()
}

func (f Frame) sNodeSwapLine() string {
	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(bold("swap"))
	sb.WriteString("\t\t\t")
	sb.WriteString(f.info.separator)
	sb.WriteString("\t")
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
	sb.WriteString(" ")
	sb.WriteString(bold("state"))
	sb.WriteString("\t\t\t")
	sb.WriteString(f.info.separator)
	sb.WriteString("\t")
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeStates(n))
		sb.WriteString("\t")
	}
	return sb.String()
}

func (f Frame) NodeVersions() *set.Set {
	versions := set.New()
	for _, n := range f.Current.Cluster.Config.Nodes {
		versions.Insert(f.StrNodeVersion(n))
	}
	versions.Remove(iconUndef)
	return versions
}

func (f Frame) sNodeVersionLine() string {
	versions := f.NodeVersions()
	if versions.Len() <= 1 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(bold("version"))
	sb.WriteString("\t")
	sb.WriteString(yellow("warn"))
	sb.WriteString("\t\t")
	sb.WriteString(f.info.separator)
	sb.WriteString("\t")
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeVersion(n))
		sb.WriteString("\t")
	}
	return sb.String() + "\n"
}

func (f Frame) sNodeCompatLine() string {
	if f.Current.Cluster.Status.IsCompat {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(bold("compat"))
	sb.WriteString("\t")
	sb.WriteString(yellow("warn"))
	sb.WriteString("\t\t")
	sb.WriteString(f.info.separator)
	sb.WriteString("\t")
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

func BSizeCompactFromMB(n uint64) string {
	f := float64(n * sizeconv.MiB)
	s := sizeconv.BSizeCompact(f)
	return strings.TrimSuffix(s, "i")
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
		total := BSizeCompactFromMB(val.Stats.MemTotalMB)
		var sb strings.Builder
		if val.Config.MinAvailMemPct > 0 {
			sb.WriteString(strconv.Itoa(usage))
			sb.WriteString("%")
			sb.WriteString(total)
			sb.WriteString("<")
			sb.WriteString(strconv.Itoa(limit))
			sb.WriteString("%")
		} else {
			sb.WriteString(strconv.Itoa(usage))
			sb.WriteString("%")
			sb.WriteString(total)
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
		total := BSizeCompactFromMB(val.Stats.SwapTotalMB)
		var sb strings.Builder
		if val.Config.MinAvailSwapPct > 0 {
			sb.WriteString(strconv.Itoa(usage))
			sb.WriteString("%")
			sb.WriteString(total)
			sb.WriteString("<")
			sb.WriteString(strconv.Itoa(limit))
			sb.WriteString("%")
		} else {
			sb.WriteString(strconv.Itoa(usage))
			sb.WriteString("%")
			sb.WriteString(total)
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

func (f Frame) StrNodeVersion(n string) string {
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
	sb.WriteString(" ")
	sb.WriteString(bold("hb-q"))
	sb.WriteString("\t\t\t")
	sb.WriteString(f.info.separator)
	sb.WriteString("\t")
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
	sb.WriteString(" ")
	sb.WriteString(bold("uptime"))
	sb.WriteString("\t\t\t")
	sb.WriteString(f.info.separator)
	sb.WriteString("\t")
	for _, n := range f.Current.Cluster.Config.Nodes {
		sb.WriteString(f.StrNodeUptime(n))
		sb.WriteString("\t")
	}
	return sb.String()
}

func (f Frame) StrNodeUptime(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		bt := val.Status.BootedAt
		if bt.IsZero() {
			return yellow("-")
		}
		diffTime := now().Sub(val.Status.BootedAt)
		return duration.FmtShortDuration(diffTime)
	}
	return iconUndef
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
