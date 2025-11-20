package monitor

import (
	"fmt"
	"strings"
)

func (f Frame) sDaemonStateLine() string {
	s := fmt.Sprintf(" %s\t\t\t%s\t", bold("state"), f.info.separator)
	for _, node := range f.Current.Cluster.Config.Nodes {
		s += f.StrDaemonState(node) + "\t"
	}
	return s
}

func (f Frame) StrDaemonState(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		if val.Daemon.Dns.State != "running" {
			return iconDownIssue
		}
		s := val.Daemon.Dns.ID
		switch f.Current.Cluster.Node[n].Daemon.Collector.State {
		case "speaker":
			s += ",speaker"
		case "speaker-warning":
			s += ",speaker" + iconWarning
		default:
		}
		return s
	}
	return iconUndef
}

func (f Frame) sHbQueueLine() string {
	s := fmt.Sprintf(" %s\t\t\t%s\t", bold("hb queue"), f.info.separator)
	for _, node := range f.Current.Cluster.Config.Nodes {
		s += f.StrHbQueue(node) + "\t"
	}
	return s
}

func (f Frame) StrHbQueue(n string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		mode := ""
		nodeCount := len(f.Current.Cluster.Config.Nodes)
		lastMessage := val.Daemon.Heartbeat.LastMessage
		switch lastMessage.Type {
		case "patch":
			mode = fmt.Sprintf("%d", lastMessage.PatchLength)
		default:
			mode = lastMessage.Type
		}
		switch mode {
		case "full":
			mode = yellow(mode)
		case "ping":
			if nodeCount > 1 {
				mode = yellow(mode)
			}
		case "":
			if nodeCount > 1 {
				mode = hired("?")
			} else {
				mode = "?"
			}
		}
		return mode
	}
	return iconUndef
}

func (f Frame) sHeartbeatLine(hbType string) string {
	s := fmt.Sprintf(" %s\t\t\t%s\t", bold("hb "+hbType), f.info.separator)

	for _, node := range f.Current.Cluster.Config.Nodes {
		s += f.StrHeartbeat(node, hbType) + "\t"
	}

	return s
}

func (f Frame) StrHeartbeat(n string, hbType string) string {
	if val, ok := f.Current.Cluster.Node[n]; ok {
		valid := 0
		total := 0
		for _, stream := range val.Daemon.Heartbeat.Streams {
			if !strings.Contains(stream.ID, hbType) {
				continue
			}
			for _, peer := range f.Current.Cluster.Config.Nodes {
				if peer == n {
					continue
				}
				total++
				peerData, ok := stream.Peers[peer]
				if !ok {
					continue
				}
				if peerData.IsBeating {
					valid++
				}
			}
		}
		if total == 0 {
			return iconNotApplicable
		}
		if valid == total {
			return iconUp
		} else if valid > 0 {
			return iconUp + iconWarning
		} else {
			return iconDownIssue
		}
	}
	return ""
}

func (f Frame) wDaemons() {
	fmt.Fprintln(f.w, f.title("Daemon"))
	fmt.Fprintln(f.w, f.sUptimeLine())
	fmt.Fprintln(f.w, f.sDaemonStateLine())
	fmt.Fprintln(f.w, f.sHbQueueLine())
	fmt.Fprintln(f.w, f.sHeartbeatLine("rx"))
	fmt.Fprintln(f.w, f.sHeartbeatLine("tx"))
	fmt.Fprintln(f.w, f.info.empty)

}
