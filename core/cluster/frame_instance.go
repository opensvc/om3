package cluster

import (
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/stringslice"
)

func (f Frame) sObjectInstance(path string, node string) string {
	s := ""
	avail := f.Current.Cluster.Object[path].Avail
	if inst, ok := f.Current.Cluster.Node[node].Instance[path]; ok {
		if inst.Status != nil {
			var instanceMonitor instance.Monitor
			if inst.Monitor != nil {
				instanceMonitor = *inst.Monitor
			} else {
				instanceMonitor = instance.Monitor{}
			}
			instanceStatus := *inst.Status
			s += sObjectInstanceAvail(avail, instanceStatus)
			s += sObjectInstanceOverall(instanceStatus)
			s += sObjectInstanceDRP(instanceStatus)
			s += sObjectInstanceLeader(instanceMonitor)
			s += sObjectInstanceFrozen(instanceStatus)
			s += sObjectInstanceUnprovisioned(instanceStatus)
			s += sObjectInstanceMonitorStatus(instanceMonitor)
			s += sObjectInstanceMonitorGlobalExpect(instanceMonitor)
		} else if localInst, ok := f.Current.Cluster.Node[hostname.Hostname()].Instance[path]; !ok || localInst.Config == nil {
		} else if stringslice.Has(node, localInst.Config.Scope) {
			s += iconUndef
		}
	} else {
		s += iconUndef
	}
	return s + "\t"
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
			return iconDown
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

func sObjectInstanceLeader(instanceMonitor instance.Monitor) string {
	if instanceMonitor.IsLeader {
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

func sObjectInstanceMonitorStatus(instanceMonitor instance.Monitor) string {
	if instanceMonitor.Status != "idle" {
		return " " + instanceMonitor.Status
	}
	return ""
}

func sObjectInstanceMonitorGlobalExpect(instanceMonitor instance.Monitor) string {
	if instanceMonitor.GlobalExpect != "" {
		return hiblack(" >" + instanceMonitor.GlobalExpect)
	}
	return ""
}
