package cluster

import (
	"fmt"

	"opensvc.com/opensvc/core/status"
)

func (f Frame) wObjects() {
	fmt.Fprintln(f.w, f.title("Objects"))
	for _, k := range f.info.paths {
		fmt.Fprintln(f.w, f.sObject(k))
	}
}

func sObjectPlacement(d ServiceStatus) string {
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

func sObjectWarning(d ServiceStatus) string {
	var s string
	if d.Overall == status.Warn {
		s = iconWarning
	}
	return s
}

func (f Frame) sObjectRunning(path string) string {
	actual := 0
	expected := 0
	orchestrate := ""
	for _, node := range f.Current.Monitor.Nodes {
		if instance, ok := node.Services.Status[path]; ok {
			if instance.Avail == status.Up {
				actual++
			}
			if expected == 0 {
				if instance.Topology == "flex" {
					expected = instance.FlexTarget
				}
				if instance.Topology == "failover" {
					expected = 1
				}

			}
			orchestrate = instance.Orchestrate
		}
	}
	if actual == 0 && expected == 0 {
		return ""
	}
	if expected == 0 {
		return fmt.Sprintf("%-5s %d", orchestrate, actual)
	}
	return fmt.Sprintf("%-5s %d/%d", orchestrate, actual, expected)
}

func sObjectAvail(d ServiceStatus) string {
	s := d.Avail
	return s.ColorString()
}

func (f Frame) sObject(path string) string {
	d := f.Current.Monitor.Services[path]
	c3 := sObjectAvail(d) + sObjectWarning(d) + sObjectPlacement(d)
	s := fmt.Sprintf(" %s\t", bold(path))
	s += fmt.Sprintf("%s\t", c3)
	s += fmt.Sprintf("%s\t", f.sObjectRunning(path))
	s += fmt.Sprintf("%s\t", f.info.separator)
	for _, node := range f.Current.Cluster.Nodes {
		s += f.sObjectInstance(path, node)
	}
	return s
}
