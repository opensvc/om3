package monitor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/status"
)

func (f Frame) wArbitrators() {
	if len(f.info.arbitrators) == 0 {
		return
	}
	var sb strings.Builder
	sb.WriteString(bold("arbitrators"))
	sb.WriteString("\t\t\t")
	sb.WriteString(f.info.separator)
	sb.WriteString("\t")
	sb.WriteString(f.info.emptyNodes)
	sb.WriteString("\n")
	arbitrators := make([]string, 0)
	for name := range f.info.arbitrators {
		arbitrators = append(arbitrators, name)
	}
	sort.Strings(arbitrators)
	for _, name := range arbitrators {
		for i, node := range f.Current.Cluster.Config.Nodes {
			if i == 0 {
				sb.WriteString(bold(name))
				sb.WriteString("\t\t\t")
				sb.WriteString(f.info.separator)
				sb.WriteString("\t")
			}
			sb.WriteString(f.StrNodeArbitratorStatus(name, node))
			sb.WriteString("\t")
		}
		sb.WriteString("\n")
	}
	_, _ = fmt.Fprint(f.w, sb.String())
	_, _ = fmt.Fprintln(f.w, f.info.empty)
}

func (f Frame) StrNodeArbitratorStatus(name, node string) string {
	s := ""
	aStatus := f.Current.Cluster.Node[node].Status.Arbitrators[name].Status
	switch aStatus {
	case status.Up:
		s += iconUp
	default:
		s += iconDown
	}
	return s
}
