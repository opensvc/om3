package cluster

import (
	"fmt"
	"io"

	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/types"
)

func wObjects(w io.Writer, data Data, info *dataInfo) {
	fmt.Fprintln(w, title("Objects", data))
	for _, k := range info.paths {
		fmt.Fprintln(w, sObject(k, data, info))
	}
}

func sObjectPlacement(d types.ServiceStatus) string {
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

func sObjectWarning(d types.ServiceStatus) string {
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

func sObjectAvail(d types.ServiceStatus) string {
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

func sObjectInstance(path string, node string, data Data) string {
	s := ""
	avail := data.Current.Monitor.Services[path].Avail
	if instance, ok := data.Current.Monitor.Nodes[node].Services.Status[path]; ok {
		s += sObjectInstanceAvail(avail, instance)
		s += sObjectInstanceOverall(instance)
		s += sObjectInstanceDRP(instance)
		s += sObjectInstanceLeader(instance)
		s += sObjectInstanceFrozen(instance)
		s += sObjectInstanceUnprovisioned(instance)
		s += sObjectInstanceMonitorStatus(instance)
		s += sObjectInstanceMonitorGlobalExpect(instance)
	}
	return s
}

func sObjectInstanceAvail(avail status.Type, instance types.InstanceStatus) string {
	switch instance.Avail {
	case status.Up:
		return iconUp
	case status.Down:
		return iconDown
	case status.Warn:
		return iconWarning
	case status.NotApplicable:
		return iconNotApplicable
	case status.StandbyDown:
		return iconStandbyDown
	case status.StandbyUp:
		if avail != status.Up {
			return iconStandbyUpIssue
		}
		return iconStandbyUp
	}
	return instance.Avail.String()
}

func sObjectInstanceOverall(instance types.InstanceStatus) string {
	if instance.Overall == status.Warn {
		return iconWarning
	}
	return ""
}

func sObjectInstanceDRP(instance types.InstanceStatus) string {
	if instance.DRP {
		return iconDRP
	}
	return ""
}

func sObjectInstanceLeader(instance types.InstanceStatus) string {
	if instance.Placement == "leader" {
		return iconLeader
	}
	return ""
}

func sObjectInstanceFrozen(instance types.InstanceStatus) string {
	if instance.Frozen > 0 {
		return iconFrozen
	}
	return ""
}

func sObjectInstanceUnprovisioned(instance types.InstanceStatus) string {
	if !instance.Provisioned {
		return iconProvisionAlert
	}
	return ""
}

func sObjectInstanceMonitorStatus(instance types.InstanceStatus) string {
	if instance.Monitor.Status != "idle" {
		return " " + instance.Monitor.Status
	}
	return ""
}

func sObjectInstanceMonitorGlobalExpect(instance types.InstanceStatus) string {
	if instance.Monitor.GlobalExpect != "" {
		return hiblack(" >" + instance.Monitor.GlobalExpect)
	}
	return ""
}
