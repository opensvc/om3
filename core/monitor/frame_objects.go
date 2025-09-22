package monitor

import (
	"fmt"
	"strings"

	"github.com/opensvc/om3/core/colorstatus"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
)

func (f Frame) wObjects() {
	s := "Objects"
	if f.Selector != "" {
		s += " matching " + f.Selector
	}
	fmt.Fprintln(f.w, f.title(s))
	for _, k := range f.info.paths {
		fmt.Fprintln(f.w, f.sObject(k))
	}
}

func sObjectPlacement(d object.Status) string {
	if d.ActorStatus == nil {
		return ""
	}
	if d.ActorStatus.PlacementState == placement.NonOptimal {
		return iconPlacementAlert
	}
	return ""
}

func sObjectWarning(d object.Status) string {
	if d.ActorStatus == nil {
		return ""
	}
	if d.ActorStatus.Overall == status.Warn {
		return iconWarning
	}
	return ""
}

func (f Frame) scalerInstancesUp(path string) int {
	var actual int
	for _, node := range f.Current.Cluster.Node {
		for p, inst := range node.Instance {
			if inst.Status == nil {
				continue
			}
			l := strings.SplitN(p, ".", 2)
			if len(l) == 2 && l[1] == path && inst.Status.Avail == status.Up {
				actual++
			}
		}
	}
	return actual
}

func (f Frame) sObjectOrchestrateAndRunning(path string) string {
	s := f.Current.Cluster.Object[path]
	if s.ActorStatus == nil {
		return ""
	}
	return fmt.Sprintf("%-5s %s", s.Orchestrate, f.StrObjectRunning(path))
}

func (f Frame) StrObjectRunning(path string) string {
	var (
		actual, expected int
	)
	avail := status.NotApplicable

	objectStatus, ok := f.Current.Cluster.Object[path]
	if objectStatus.ActorStatus == nil {
		return ""
	}
	if ok {
		avail = objectStatus.Avail
	}

	for _, node := range f.Current.Cluster.Node {
		if inst, ok := node.Instance[path]; ok {
			if inst.Status == nil {
				continue
			}
			instanceStatus := *inst.Status
			if instanceStatus.Avail == status.Up {
				actual++
			}
			if expected == 0 {
				switch {
				//case !instanceStatus.Scale.IsZero():
				//	expected = int(instanceStatus.Scale.ValueOrZero())
				case objectStatus.Topology == topology.Flex:
					expected = objectStatus.Flex.Target
				case objectStatus.Topology == topology.Failover:
					expected = 1
				}
			}
		}
	}

	switch {
	case actual == 0 && expected == 0:
		return ""
	case expected == 0:
		return fmt.Sprintf("%d", actual)
	case avail == status.NotApplicable:
		return ""
	default:
		return fmt.Sprintf("%d/%d", actual, expected)
	}
}

func StrObjectStatus(d object.Status) string {
	return sObjectAvail(d) + sObjectWarning(d) + sObjectPlacement(d)
}

func sObjectAvail(d object.Status) string {
	if d.ActorStatus == nil {
		return ""
	}
	return colorstatus.Sprint(d.ActorStatus.Avail, rawconfig.Colorize)
}

func (f Frame) sObject(path string) string {
	d := f.Current.Cluster.Object[path]
	s := fmt.Sprintf(" %s\t", bold(path))
	s += fmt.Sprintf("%s\t", StrObjectStatus(d))
	s += fmt.Sprintf("%s\t", f.sObjectOrchestrateAndRunning(path))
	s += fmt.Sprintf("%s\t", f.info.separator)
	for _, node := range f.Current.Cluster.Config.Nodes {
		s += f.StrObjectInstance(path, node, d.Scope) + "\t"
	}
	return s
}
