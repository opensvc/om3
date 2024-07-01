package cluster

import (
	"fmt"

	"github.com/opensvc/om3/daemon/dsubsystem"
	"github.com/opensvc/om3/util/render/listener"
)

func (f Frame) wThreadDaemon() string {
	var s string
	s += bold(" daemon") + "\t"
	s += green("running") + "\t"
	s += "\t"
	s += f.info.separator + "\t"
	s += f.info.emptyNodes
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
	s += bold(" listener") + "\t"
	if f.Current.Cluster.Node[f.Current.Daemon.Nodename].Status.Lsnr.Port != "" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += fmt.Sprintf("%s\t", listener.Render(f.Current.Daemon.Listener.Config.Addr, f.Current.Cluster.Config.Listener.Port))
	s += f.info.separator + "\t"
	for _, node := range f.Current.Cluster.Config.Nodes {
		port := f.Current.Cluster.Node[node].Status.Lsnr.Port
		switch port {
		case "":
			s += iconDownIssue + "\t"
		default:
			s += port + "\t"
		}
	}
	return s
}

func (f Frame) wThreadScheduler() string {
	var s string
	s += bold(" scheduler") + "\t"
	if f.Current.Daemon.Scheduler.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += f.info.separator + "\t"
	s += f.info.emptyNodes
	return s
}

func (f Frame) wThreadMonitor() string {
	var s string
	s += bold(" monitor") + "\t"
	if f.Current.Daemon.Daemondata.State == "running" {
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
	s += bold(" dns") + "\t"
	if f.Current.Daemon.Dns.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += f.info.separator + "\t"
	s += f.info.emptyNodes
	return s
}

func (f Frame) wThreadHeartbeats() string {
	s := fmt.Sprintf(" %s\t\t\t%s", bold("hb"), f.info.separator+"\t")
	s += f.info.emptyNodes
	for _, hbStatus := range f.Current.Daemon.Hb.Streams {
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
			if peer == f.Current.Daemon.Nodename {
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

func sThreadAlerts(data []dsubsystem.ThreadAlert) string {
	if len(data) > 0 {
		return yellow("!")
	}
	return ""
}

func (f Frame) wThreads() {
	fmt.Fprintln(f.w, f.title("Threads"))
	fmt.Fprintln(f.w, f.wThreadDaemon())
	fmt.Fprintln(f.w, f.wThreadDNS())
	fmt.Fprintln(f.w, f.wThreadCollector())
	fmt.Fprintln(f.w, f.wThreadHeartbeats())
	fmt.Fprintln(f.w, f.wThreadListener())
	fmt.Fprintln(f.w, f.wThreadMonitor())
	fmt.Fprintln(f.w, f.wThreadScheduler())
	fmt.Fprintln(f.w, f.info.empty)
}
