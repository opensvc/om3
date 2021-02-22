package cluster

import (
	"fmt"
	"io"

	"opensvc.com/opensvc/core/status"
)

func wObjects(w io.Writer, data Data, info *dataInfo) {
	fmt.Fprintln(w, title("Objects", data))
	for _, k := range info.paths {
		fmt.Fprintln(w, sObject(k, data, info))
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

func sObjectRunning(path string, data Data) string {
	actual := 0
	expected := 0
	orchestrate := ""
	for _, node := range data.Current.Monitor.Nodes {
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

func sObject(path string, data Data, info *dataInfo) string {
	d := data.Current.Monitor.Services[path]
	c3 := sObjectAvail(d) + sObjectWarning(d) + sObjectPlacement(d)
	s := fmt.Sprintf(" %s\t", bold(path))
	s += fmt.Sprintf("%s\t", c3)
	s += fmt.Sprintf("%s\t", sObjectRunning(path, data))
	s += fmt.Sprintf("%s\t", info.separator)
	for _, node := range data.Current.Cluster.Nodes {
		s += sObjectInstance(path, node, data)
	}
	return s
}
