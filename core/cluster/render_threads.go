package cluster

import (
	"fmt"

	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/render/listener"
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
	s += bold(" collector") + "\t"
	if f.Current.Collector.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += f.info.separator + "\t"
	for _, v := range f.Current.Monitor.Nodes {
		if v.Speaker {
			s += green("O") + "\t"
		} else {
			s += "\t"
		}
	}
	return s
}

func (f Frame) wThreadListener() string {
	var s string
	s += bold(" listener") + "\t"
	if f.Current.Listener.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += fmt.Sprintf("%s\t", listener.Render(f.Current.Listener.Config.Addr, f.Current.Listener.Config.Port))
	s += f.info.separator + "\t"
	s += f.info.emptyNodes
	return s
}

func (f Frame) wThreadScheduler() string {
	var s string
	s += bold(" scheduler") + "\t"
	if f.Current.Scheduler.State == "running" {
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
	if f.Current.Monitor.State == "running" {
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
	if f.Current.DNS.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += f.info.separator + "\t"
	s += f.info.emptyNodes
	return s
}

func (f Frame) wThreadHeartbeat(name string, data HeartbeatThreadStatus) string {
	var s string
	s += bold(" "+name) + "\t"
	if data.State == "running" {
		s += green("running") + sThreadAlerts(data.Alerts) + "\t"
	} else {
		s += red("stopped") + sThreadAlerts(data.Alerts) + "\t"
	}
	s += "\t"
	s += f.info.separator + "\t"
	for _, peer := range f.Current.Cluster.Config.Nodes {
		if peer == hostname.Hostname() {
			s += iconNotApplicable + "\t"
			continue
		}
		hb, ok := f.Current.Heartbeats[name]
		if !ok {
			s += iconUndef + "\t"
			continue
		}
		peerData, ok := hb.Peers[peer]
		if !ok {
			s += iconUndef + "\t"
			continue
		}
		if peerData.Beating {
			s += iconUp + "\t"
		} else {
			s += iconDownIssue + "\t"
		}
	}
	return s
}

func sThreadAlerts(data []ThreadAlert) string {
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
	for k, v := range f.Current.Heartbeats {
		fmt.Fprintln(f.w, f.wThreadHeartbeat(k, v))
	}
	fmt.Fprintln(f.w, f.wThreadListener())
	fmt.Fprintln(f.w, f.wThreadMonitor())
	fmt.Fprintln(f.w, f.wThreadScheduler())
	fmt.Fprintln(f.w, f.info.empty)
}
