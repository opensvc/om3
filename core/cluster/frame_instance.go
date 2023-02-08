package cluster

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/stringslice"
)

func (f Frame) sObjectInstance(path string, node string, scope []string) string {
	s := ""
	avail := f.Current.Cluster.Object[path].Avail
	inst := f.Current.Cluster.Node[node].Instance[path]
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
		s += sObjectInstanceHALeader(instanceMonitor)
		s += sObjectInstanceFrozen(instanceStatus)
		s += sObjectInstanceUnprovisioned(instanceStatus)
		s += sObjectInstanceMonitorState(instanceMonitor)
		s += sObjectInstanceMonitorGlobalExpect(instanceMonitor)
	} else if inst.Config != nil || stringslice.Has(node, scope) {
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

func sObjectInstanceHALeader(instanceMonitor instance.Monitor) string {
	if instanceMonitor.IsHALeader {
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

func sObjectInstanceMonitorState(instanceMonitor instance.Monitor) string {
	if instanceMonitor.State != instance.MonitorStateIdle {
		return " " + instanceMonitor.State.String()
	}
	return ""
}

func sObjectInstanceMonitorGlobalExpect(instanceMonitor instance.Monitor) string {
	if instanceMonitor.GlobalExpect != instance.MonitorGlobalExpectUnset {
		return hiblack(" >" + instanceMonitor.GlobalExpect.String())
	}
	return ""
}
