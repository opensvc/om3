package cluster

import (
	"fmt"

	"github.com/golang-collections/collections/set"

	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/sizeconv"
)

func (f Frame) sNodeScoreLine() string {
	s := fmt.Sprintf(" %s\t\t\t%s\t", bold("score"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.sNodeScore(n) + "\t"
	}
	return s
}
func (f Frame) sNodeLoadLine() string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("load15m"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.sNodeLoad(n) + "\t"
	}
	return s
}

func (f Frame) sNodeMemLine() string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("mem"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.sNodeMem(n) + "\t"
	}
	return s
}

func (f Frame) sNodeSwapLine() string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("swap"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.sNodeSwap(n) + "\t"
	}
	return s
}

func (f Frame) sNodeWarningsLine() string {
	s := fmt.Sprintf("%s\t\t\t%s\t", bold("state"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.sNodeMonState(n)
		s += f.sNodeFrozen(n)
		s += f.sNodeMonTarget(n)
		s += "\t"
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
	if f.Current.Cluster.Status.Compat {
		return ""
	}
	s := fmt.Sprintf("  %s\t%s\t\t%s\t", bold("compat"), yellow("warn"), f.info.separator)
	for _, n := range f.Current.Cluster.Config.Nodes {
		s += f.sNodeCompat(n) + "\t"
	}
	return s + "\n"
}

func (f Frame) sNodeScore(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		return fmt.Sprintf("%d", val.Stats.Score)
	}
	return ""
}

func (f Frame) sNodeLoad(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		return fmt.Sprintf("%.1f", val.Stats.Load15M)
	}
	return ""
}

func (f Frame) sNodeMem(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
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

func (f Frame) sNodeSwap(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
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

func (f Frame) sNodeMonState(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		if val.Monitor.Status != "idle" {
			return val.Monitor.Status
		}
	}
	return ""
}

func (f Frame) sNodeFrozen(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		if !val.Frozen.IsZero() {
			return iconFrozen
		}
	}
	return ""
}

func (f Frame) sNodeMonTarget(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		if val.Monitor.GlobalExpect != "" {
			return rawconfig.Colorize.Secondary(" >" + val.Monitor.GlobalExpect)
		}
	}
	return ""
}

func (f Frame) sNodeCompat(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		return fmt.Sprintf("%d", val.Compat)
	}
	return ""
}

func (f Frame) sNodeVersion(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		return fmt.Sprintf("%s", val.Agent)
	}
	return ""
}

func (f Frame) wNodes() {
	fmt.Fprintln(f.w, f.title("Nodes"))
	fmt.Fprintln(f.w, f.sNodeScoreLine())
	fmt.Fprintln(f.w, f.sNodeLoadLine())
	fmt.Fprintln(f.w, f.sNodeMemLine())
	fmt.Fprintln(f.w, f.sNodeSwapLine())
	fmt.Fprint(f.w, f.sNodeVersionLine())
	fmt.Fprint(f.w, f.sNodeCompatLine())
	fmt.Fprintln(f.w, f.sNodeWarningsLine())
	fmt.Fprintln(f.w, f.info.empty)
}
