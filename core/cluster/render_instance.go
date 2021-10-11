package cluster

import (
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
)

func (f Frame) sObjectInstance(path string, node string) string {
	s := ""
	avail := f.Current.Monitor.Services[path].Avail
	if instance, ok := f.Current.Monitor.Nodes[node].Services.Status[path]; ok {
		s += sObjectInstanceAvail(avail, instance)
		s += sObjectInstanceOverall(instance)
		s += sObjectInstanceDRP(instance)
		s += sObjectInstanceLeader(instance)
		s += sObjectInstanceFrozen(instance)
		s += sObjectInstanceUnprovisioned(instance)
		s += sObjectInstanceMonitorStatus(instance)
		s += sObjectInstanceMonitorGlobalExpect(instance)
		s += "\t"
	}
	return s
}

func sObjectInstanceAvail(objectAvail status.T, instance instance.Status) string {
	if instance.Preserved {
		return iconPreserved
	}
	switch instance.Avail {
	case status.Undef:
		return iconUndef
	case status.Up:
		return iconUp
	case status.Down:
		if objectAvail == status.Up {
			return iconDownIssue
		}
		return iconDownIssue
	case status.Warn:
		return iconWarning
	case status.NotApplicable:
		return iconNotApplicable
	case status.StandbyDown:
		return iconStandbyDown
	case status.StandbyUp:
		if objectAvail == status.Up {
			return iconStandbyUp
		}
		return iconStandbyUpIssue
	}
	return instance.Avail.String()
}

func sObjectInstanceOverall(instance instance.Status) string {
	if instance.Overall == status.Warn {
		return iconWarning
	}
	return ""
}

func sObjectInstanceDRP(instance instance.Status) string {
	if instance.DRP {
		return iconDRP
	}
	return ""
}

func sObjectInstanceLeader(instance instance.Status) string {
	if instance.Monitor.Placement == "leader" {
		return iconLeader
	}
	return ""
}

func sObjectInstanceFrozen(instance instance.Status) string {
	if !instance.Frozen.IsZero() {
		return iconFrozen
	}
	return ""
}

func sObjectInstanceUnprovisioned(instance instance.Status) string {
	switch instance.Provisioned {
	case provisioned.False:
		return iconProvisionAlert
	case provisioned.Mixed:
		return iconProvisionAlert
	default:
		return ""
	}
}

func sObjectInstanceMonitorStatus(instance instance.Status) string {
	if instance.Monitor.Status != "idle" {
		return " " + instance.Monitor.Status
	}
	return ""
}

func sObjectInstanceMonitorGlobalExpect(instance instance.Status) string {
	if instance.Monitor.GlobalExpect != "" {
		return hiblack(" >" + instance.Monitor.GlobalExpect)
	}
	return ""
}
