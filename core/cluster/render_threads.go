package cluster

import (
	"fmt"
	"io"

	"opensvc.com/opensvc/util/render/listener"
)

func wThreadDaemon(data Data, info *dataInfo) string {
	var s string
	s += bold(" daemon") + "\t"
	s += green("running") + "\t"
	s += "\t"
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func wThreadCollector(data Data, info *dataInfo) string {
	var s string
	s += bold(" collector") + "\t"
	if data.Current.Collector.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += info.separator + "\t"
	for _, v := range data.Current.Monitor.Nodes {
		if v.Speaker {
			s += green("O") + "\t"
		} else {
			s += "\t"
		}
	}
	return s
}

func wThreadListener(data Data, info *dataInfo) string {
	var s string
	s += bold(" listener") + "\t"
	if data.Current.Listener.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += fmt.Sprintf("%s\t", listener.Render(data.Current.Listener.Config.Addr, data.Current.Listener.Config.Port))
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func wThreadScheduler(data Data, info *dataInfo) string {
	var s string
	s += bold(" scheduler") + "\t"
	if data.Current.Scheduler.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func wThreadMonitor(data Data, info *dataInfo) string {
	var s string
	s += bold(" monitor") + "\t"
	if data.Current.Monitor.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func wThreadDNS(data Data, info *dataInfo) string {
	var s string
	s += bold(" dns") + "\t"
	if data.Current.DNS.State == "running" {
		s += green("running") + "\t"
	} else {
		s += "\t"
	}
	s += "\t"
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func wThreadHeartbeat(name string, data HeartbeatThreadStatus, info *dataInfo) string {
	var s string
	s += bold(" "+name) + "\t"
	if data.State == "running" {
		s += green("running") + sThreadAlerts(data.Alerts) + "\t"
	} else {
		s += red("stopped") + sThreadAlerts(data.Alerts) + "\t"
	}
	s += "\t"
	s += info.separator + "\t"
	s += info.emptyNodes
	return s
}

func sThreadAlerts(data []ThreadAlert) string {
	if len(data) > 0 {
		return yellow("!")
	}
	return ""
}

func wThreads(w io.Writer, data Data, info *dataInfo) {
	fmt.Fprintln(w, title("Threads", data))
	fmt.Fprintln(w, wThreadDaemon(data, info))
	fmt.Fprintln(w, wThreadDNS(data, info))
	fmt.Fprintln(w, wThreadCollector(data, info))
	for k, v := range data.Current.Heartbeats {
		fmt.Fprintln(w, wThreadHeartbeat(k, v, info))
	}
	fmt.Fprintln(w, wThreadListener(data, info))
	fmt.Fprintln(w, wThreadMonitor(data, info))
	fmt.Fprintln(w, wThreadScheduler(data, info))
	fmt.Fprintln(w, info.empty)
}
