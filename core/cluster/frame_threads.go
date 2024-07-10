package cluster

import (
	"fmt"

	"github.com/opensvc/om3/daemon/daemonsubsystem"
)

func (f Frame) wThreadDaemon() string {
	var s string
	s += bold(" daemon") + "\t\t\t"
	s += f.info.separator + "\t"
	for _, node := range f.Current.Cluster.Config.Nodes {
		switch f.Current.Cluster.Node[node].Daemon.Daemondata.State {
		case "running":
			s += iconUp
		default:
			s += iconUndef
		}
		s += "\t"
	}
	return s
}

func (f Frame) wThreadCollector() string {
	var s string
	s += bold(" collector") + "\t\t\t"
	s += f.info.separator + "\t"
	for _, node := range f.Current.Cluster.Config.Nodes {
		switch f.Current.Cluster.Node[node].Daemon.Collector.State {
		case "speaker":
			s += iconUp
		case "speaker-candidate":
			s += iconStandbyUp
		case "speaker-warning":
			s += iconDownIssue
		case "warning":
			s += iconStandbyDown
		case "disabled":
			s += iconNotApplicable
		default:
			s += iconUndef
		}
		s += "\t"
	}
	return s
}

func (f Frame) wThreadListener() string {
	var s string
	s += bold(" listener") + "\t\t\t" + f.info.separator + "\t"
	for _, node := range f.Current.Cluster.Config.Nodes {
		lsnr := f.Current.Cluster.Node[node].Daemon.Listener
		if lsnr.State != "running" {
			s += iconDownIssue
		} else if lsnr.Port == "" {
			s += iconWarning
		} else {
			s += lsnr.Port
		}
		s += "\t"
	}
	return s
}

func (f Frame) wThreadScheduler() string {
	var s string
	s += bold(" scheduler") + "\t"
	if f.Current.Cluster.Node[f.Nodename].Daemon.Scheduler.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += f.info.separator + "\t"
	s += f.info.emptyNodes
	return s
}

func (f Frame) wThreadDNS() string {
	var s string
	s += bold(" dns") + "\t\t\t" + f.info.separator + "\t"
	for _, peer := range f.Current.Cluster.Config.Nodes {
		switch f.Current.Cluster.Node[peer].Daemon.Dns.State {
		case "running":
			s += iconUp
		default:
			s += iconUndef
		}
		s += "\t"
	}
	return s
}

func (f Frame) wThreadHeartbeats() string {
	s := fmt.Sprintf(" %s\t\t\t%s", bold("hb"), f.info.separator+"\t")
	s += f.info.emptyNodes
	for _, hbStatus := range f.Current.Cluster.Node[f.Nodename].Daemon.Heartbeat.Streams {
		name := hbStatus.ID
		s += bold("\n  "+name) + "\t"
		switch hbStatus.State {
		case "running":
			s += green("running") + sThreadAlerts(hbStatus.Alerts)
		case "stopped":
			s += red("stopped") + sThreadAlerts(hbStatus.Alerts)
		case "failed":
			s += red("failed") + sThreadAlerts(hbStatus.Alerts)
		default:
			s += red("unknown") + sThreadAlerts(hbStatus.Alerts)
		}
		s += "\t" + hbStatus.Type + "\t"
		s += f.info.separator + "\t"
		for _, peer := range f.Current.Cluster.Config.Nodes {
			if peer == f.Nodename {
				s += iconNotApplicable + "\t"
				continue
			}
			peerData, ok := hbStatus.Peers[peer]
			if !ok {
				s += iconUndef + "\t"
				continue
			}
			if peerData.IsBeating {
				s += iconUp + "\t"
			} else {
				s += iconDownIssue + "\t"
			}
		}
	}

	return s
}

func sThreadAlerts(data []daemonsubsystem.Alert) string {
	if len(data) > 0 {
		return yellow("!")
	}
	return ""
}

func (f Frame) wThreads() {
	fmt.Fprintln(f.w, f.title("Threads"))
	fmt.Fprintln(f.w, f.wThreadDaemon())
	if len(f.Current.Cluster.Config.DNS) > 0 {
		fmt.Fprintln(f.w, f.wThreadDNS())
	}
	fmt.Fprintln(f.w, f.wThreadCollector())
	fmt.Fprintln(f.w, f.wThreadHeartbeats())
	fmt.Fprintln(f.w, f.wThreadListener())
	fmt.Fprintln(f.w, f.wThreadScheduler())
	fmt.Fprintln(f.w, f.info.empty)
}
