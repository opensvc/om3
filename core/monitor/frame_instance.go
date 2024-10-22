package monitor

import (
	"slices"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
)

func (f Frame) StrObjectInstance(path string, node string, scope []string) string {
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
		var instanceConfig instance.Config
		if inst.Config != nil {
			instanceConfig = *inst.Config
		} else {
			instanceConfig = instance.Config{}
		}
		instanceStatus := *inst.Status
		s += sObjectInstanceAvail(avail, instanceStatus, instanceMonitor)
		s += sObjectInstanceOverall(instanceStatus)
		s += sObjectInstanceDRP(instanceConfig)
		s += sObjectInstanceHALeader(instanceMonitor)
		s += sObjectInstanceFrozen(instanceStatus)
		s += sObjectInstanceUnprovisioned(instanceStatus)
		s += sObjectInstanceMonitorState(instanceMonitor)
		s += sObjectInstanceMonitorGlobalExpect(instanceMonitor)
	} else if inst.Config != nil || slices.Contains(scope, node) {
		s += iconUndef
	}
	return s
}

func sObjectInstanceAvail(objectAvail status.T, instance instance.Status, mon instance.Monitor) string {
	if mon.IsPreserved {
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

func sObjectInstanceDRP(instance instance.Config) string {
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
	if !instance.FrozenAt.IsZero() {
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
	switch instanceMonitor.State {
	case instance.MonitorStateInit:
		return ""
	case instance.MonitorStateIdle:
		return ""
	default:
		return " " + instanceMonitor.State.String()
	}
}

func sObjectInstanceMonitorGlobalExpect(instanceMonitor instance.Monitor) string {
	switch instanceMonitor.GlobalExpect {
	case instance.MonitorGlobalExpectInit:
		return ""
	case instance.MonitorGlobalExpectNone:
		return ""
	default:
		return hiblack(" >" + instanceMonitor.GlobalExpect.String())
	}
}
