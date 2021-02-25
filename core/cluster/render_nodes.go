package cluster

import (
	"fmt"

	"opensvc.com/opensvc/util/converters/sizeconv"
)

func (f Frame) sNodeScoreLine() string {
	s := fmt.Sprintf(" %s\t\t\t%s\t", bold("score"), f.info.separator)
	for _, n := range f.Current.Cluster.Nodes {
		s += f.sNodeScore(n) + "\t"
	}
	return s
}
func (f Frame) sNodeLoadLine() string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("load15m"), f.info.separator)
	for _, n := range f.Current.Cluster.Nodes {
		s += f.sNodeLoad(n) + "\t"
	}
	return s
}

func (f Frame) sNodeMemLine() string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("mem"), f.info.separator)
	for _, n := range f.Current.Cluster.Nodes {
		s += f.sNodeMem(n) + "\t"
	}
	return s
}

func (f Frame) sNodeSwapLine() string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("swap"), f.info.separator)
	for _, n := range f.Current.Cluster.Nodes {
		s += f.sNodeSwap(n) + "\t"
	}
	return s
}

func (f Frame) sNodeScore(n string) string {
	if val, ok := f.Current.Monitor.Nodes[n]; ok {
		return fmt.Sprintf("%d", val.Stats.Score)
	}
	return ""
}

func (f Frame) sNodeLoad(n string) string {
	if val, ok := f.Current.Monitor.Nodes[n]; ok {
		return fmt.Sprintf("%.1f", val.Stats.Load15M)
	}
	return ""
}

func (f Frame) sNodeMem(n string) string {
	if val, ok := f.Current.Monitor.Nodes[n]; ok {
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
	if val, ok := f.Current.Monitor.Nodes[n]; ok {
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

func (f Frame) wNodes() {
	fmt.Fprintln(f.w, f.title("Nodes"))
	fmt.Fprintln(f.w, f.sNodeScoreLine())
	fmt.Fprintln(f.w, f.sNodeLoadLine())
	fmt.Fprintln(f.w, f.sNodeMemLine())
	fmt.Fprintln(f.w, f.sNodeSwapLine())
	fmt.Fprintln(f.w, f.info.empty)
}
