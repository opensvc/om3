package cluster

import (
	"fmt"
	"strings"

	"github.com/guregu/null"

	"opensvc.com/opensvc/core/colorstatus"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/topology"
)

func (f Frame) wObjects() {
	fmt.Fprintln(f.w, f.title("Objects"))
	for _, k := range f.info.paths {
		fmt.Fprintln(f.w, f.sObject(k))
	}
}

func sObjectPlacement(d object.AggregatedStatus) string {
	var s string
	switch d.Placement {
	case "":
	case "n/a":
	case "optimal":
		s = ""
	default:
		s = iconPlacementAlert
	}
	return s
}

func sObjectWarning(d object.AggregatedStatus) string {
	var s string
	if d.Overall == status.Warn {
		s = iconWarning
	}
	return s
}

func (f Frame) scalerInstancesUp(path string) int {
	actual := 0
	for _, node := range f.Current.Cluster.Node {
		for p, instance := range node.Services.Status {
			l := strings.SplitN(p, ".", 2)
			if len(l) == 2 && l[1] == path && instance.Avail == status.Up {
				actual++
			}
		}
	}
	return actual
}

func (f Frame) sObjectRunning(path string) string {
	actual := 0
	expected := 0
	orchestrate := ""
	avail := status.NotApplicable

	var scale null.Int
	for _, node := range f.Current.Cluster.Node {
		if instance, ok := node.Services.Status[path]; ok {
			if instance.Avail == status.Up {
				actual++
			}
			if expected == 0 {
				switch {
				case !instance.Scale.IsZero():
					expected = int(instance.Scale.ValueOrZero())
				case instance.Topology == topology.Flex:
					expected = instance.FlexTarget
				case instance.Topology == topology.Failover:
					expected = 1
				}
			}
			orchestrate = instance.Orchestrate
			scale = instance.Scale
		}
	}

	if s, ok := f.Current.Cluster.Object[path]; ok {
		avail = s.Avail
	}

	switch {
	case actual == 0 && expected == 0:
		return ""
	case expected == 0:
		return fmt.Sprintf("%-5s %d", orchestrate, actual)
	case !scale.IsZero():
		actual = f.scalerInstancesUp(path)
		return fmt.Sprintf("%-5s %d/%d", orchestrate, actual, expected)
	case avail == status.NotApplicable:
		return fmt.Sprintf("%-5s", orchestrate)
	default:
		return fmt.Sprintf("%-5s %d/%d", orchestrate, actual, expected)
	}
}

func sObjectAvail(d object.AggregatedStatus) string {
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
		s += f.sObjectInstance(path, node)
	}
	return s
}
