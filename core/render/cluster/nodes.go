package cluster

import (
	"fmt"
	"io"

	"opensvc.com/opensvc/core/converters/sizeconv"
)

func sNodeScoreLine(data Data, info *dataInfo) string {
	s := fmt.Sprintf(" %s\t\t\t%s\t", bold("score"), info.separator)
	for _, n := range data.Current.Cluster.Nodes {
		s += sNodeScore(n, data) + "\t"
	}
	return s
}
func sNodeLoadLine(data Data, info *dataInfo) string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("load15m"), info.separator)
	for _, n := range data.Current.Cluster.Nodes {
		s += sNodeLoad(n, data) + "\t"
	}
	return s
}

func sNodeMemLine(data Data, info *dataInfo) string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("mem"), info.separator)
	for _, n := range data.Current.Cluster.Nodes {
		s += sNodeMem(n, data) + "\t"
	}
	return s
}

func sNodeSwapLine(data Data, info *dataInfo) string {
	s := fmt.Sprintf("  %s\t\t\t%s\t", bold("swap"), info.separator)
	for _, n := range data.Current.Cluster.Nodes {
		s += sNodeSwap(n, data) + "\t"
	}
	return s
}

func sNodeScore(n string, data Data) string {
	if val, ok := data.Current.Monitor.Nodes[n]; ok {
		return fmt.Sprintf("%d", val.Stats.Score)
	}
	return ""
}

func sNodeLoad(n string, data Data) string {
	if val, ok := data.Current.Monitor.Nodes[n]; ok {
		return fmt.Sprintf("%.1f", val.Stats.Load15M)
	}
	return ""
}

func sNodeMem(n string, data Data) string {
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

func sNodeSwap(n string, data Data) string {
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

func wNodes(w io.Writer, data Data, info *dataInfo) {
	fmt.Fprintln(w, title("Nodes", data))
	fmt.Fprintln(w, sNodeScoreLine(data, info))
	fmt.Fprintln(w, sNodeLoadLine(data, info))
	fmt.Fprintln(w, sNodeMemLine(data, info))
	fmt.Fprintln(w, sNodeSwapLine(data, info))
	fmt.Fprintln(w, info.empty)
}
