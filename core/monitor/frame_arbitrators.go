package monitor

import (
	"fmt"
	"sort"

	"github.com/opensvc/om3/core/status"
)

func (f Frame) wArbitrators() {
	if len(f.info.arbitrators) == 0 {
		return
	}
	s := fmt.Sprintf("%s\t\t\t%s", bold("arbitrators"), f.info.separator+"\t")
	s += f.info.emptyNodes + "\n"
	arbitrators := make([]string, 0)
	for name := range f.info.arbitrators {
		arbitrators = append(arbitrators, name)
	}
	sort.Strings(arbitrators)
	for _, name := range arbitrators {
		for i, node := range f.Current.Cluster.Config.Nodes {
			if i == 0 {
				s += bold(" "+name) + "\t\t\t" + f.info.separator + "\t"
			}
			aStatus := f.Current.Cluster.Node[node].Status.Arbitrators[name].Status
			switch aStatus {
			case status.Up:
				s += iconUp + "\t"
			default:
				s += iconDown + "\t"
			}
		}
		s += "\n"
	}
	fmt.Fprintf(f.w, s)
	fmt.Fprintln(f.w, f.info.empty)
}
