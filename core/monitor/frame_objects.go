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
	var s string
	if d.PlacementState == placement.NonOptimal {
		s = iconPlacementAlert
	}
	return s
}

func sObjectWarning(d object.Status) string {
	var s string
	if d.Overall == status.Warn {
		s = iconWarning
	}
	return s
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

func (f Frame) sObjectRunning(path string) string {
	var (
		actual, expected int
	)
	avail := status.NotApplicable

	s, ok := f.Current.Cluster.Object[path]
	if ok {
		avail = s.Avail
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
				case s.Topology == topology.Flex:
					expected = s.FlexTarget
				case s.Topology == topology.Failover:
					expected = 1
				}
			}
		}
	}

	switch {
	case actual == 0 && expected == 0:
		return ""
	case expected == 0:
		return fmt.Sprintf("%-5s %d", s.Orchestrate, actual)
	case avail == status.NotApplicable:
		return fmt.Sprintf("%-5s", s.Orchestrate)
	default:
		return fmt.Sprintf("%-5s %d/%d", s.Orchestrate, actual, expected)
	}
}

func sObjectAvail(d object.Status) string {
	s := d.Avail
	return colorstatus.Sprint(s, rawconfig.Colorize)
}

func (f Frame) sObject(path string) string {
	d := f.Current.Cluster.Object[path]
	c3 := sObjectAvail(d) + sObjectWarning(d) + sObjectPlacement(d)
	s := fmt.Sprintf(" %s\t", bold(path))
	s += fmt.Sprintf("%s\t", c3)
	s += fmt.Sprintf("%s\t", f.sObjectRunning(path))
	s += fmt.Sprintf("%s\t", f.info.separator)
	for _, node := range f.Current.Cluster.Config.Nodes {
		s += f.sObjectInstance(path, node, d.Scope)
	}
	return s
}
