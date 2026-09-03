package commoncmd

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
)

type (
	CmdDaemonHeartbeatList struct {
		Color        string
		Output       string
		NodeSelector string
		PeerSelector string
		Name         string
	}
)

// heartbeatStreamRow is a row of the heartbeat listing.
//
// The state and the beating flag travel from the node as values, and the
// icons standing for them are drawn here, where the output is. The daemon
// type carrying the values holds no escape sequence: it is published, and
// the tui reads the same values to paint cells of its own.
type heartbeatStreamRow struct {
	daemonsubsystem.HeartbeatStreamPeerStatusTableEntry
}

type heartbeatStreamRows []heartbeatStreamRow

// Unstructured returns the row the tab renderer reads: what the entry
// carries, plus the two icon columns the default output names.
func (t heartbeatStreamRow) Unstructured() map[string]any {
	m := t.HeartbeatStreamPeerStatusTableEntry.Unstructured()
	m["state_icon"] = heartbeatStateIcon(t.State)
	m["beating_icon"] = heartbeatBeatingIcon(t.IsSingleNode || t.IsBeating)
	return m
}

// heartbeatStateIcon draws the run state of a stream.
func heartbeatStateIcon(state string) string {
	switch state {
	case "running":
		return color.New(color.FgGreen).Sprint("O")
	case "stopped", "failed":
		return color.New(color.FgRed).Sprint("X")
	case "warning":
		return color.New(color.FgYellow).Sprint("!")
	default:
		return color.New(color.FgHiBlack).Sprint("?")
	}
}

// heartbeatBeatingIcon draws whether a stream is beating.
func heartbeatBeatingIcon(beating bool) string {
	if beating {
		return color.New(color.FgGreen).Sprint("O")
	}
	return color.New(color.FgRed).Sprint("X")
}

func NewCmdDaemonHeartbeatList(defaultNodeSelectorFilter string) *cobra.Command {
	options := CmdDaemonHeartbeatList{
		NodeSelector: defaultNodeSelectorFilter,
	}
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the heartbeat streams and their state",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagOutput(flags, &options.Output)
	FlagColor(flags, &options.Color)
	FlagNodeSelectorFilter(flags, &options.NodeSelector)
	FlagPeerSelectorFilter(flags, &options.PeerSelector)
	FlagDaemonHeartbeatFilter(flags, &options.Name)
	return cmd
}

func (t *CmdDaemonHeartbeatList) Run() error {
	cli, err := client.New()
	if err != nil {
		return err
	}
	getter := cli.NewGetClusterStatus()
	b, err := getter.Get()
	if err != nil {
		return err
	}
	var data clusterdump.Data
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	var peerMap, nodeMap map[string]any
	if t.NodeSelector != "" {
		nodeMap, err = nodeselector.New(t.NodeSelector, nodeselector.WithClient(cli)).ExpandMap()
		if err != nil {
			return err
		}
	}
	if t.PeerSelector != "" {
		peerMap, err = nodeselector.New(t.PeerSelector, nodeselector.WithClient(cli)).ExpandMap()
		if err != nil {
			return err
		}
	}
	if t.Name != "" && !strings.HasPrefix(t.Name, "hb#") {
		t.Name = "hb#" + t.Name
	}

	isSingleNode := len(data.Cluster.Node) == 1

	table := make(heartbeatStreamRows, 0)
	for nodename, nodeData := range data.Cluster.Node {
		if nodeMap != nil {
			if _, ok := nodeMap[nodename]; !ok {
				continue
			}
		}
		for _, e := range nodeData.Daemon.Heartbeat.Table(nodename, isSingleNode) {
			if peerMap != nil {
				if _, ok := peerMap[e.Peer]; !ok {
					continue
				}
			}
			if t.Name != "" {
				if strings.HasSuffix(t.Name, ".tx") || strings.HasSuffix(t.Name, ".rx") {
					if t.Name != e.ID {
						continue
					}
				} else {
					if !strings.HasPrefix(e.ID, t.Name) {
						continue
					}
				}
			}
			table = append(table, heartbeatStreamRow{e})
		}
	}

	sort.Slice(table, func(i, j int) bool {
		if table[i].Node != table[j].Node {
			return table[i].Node < table[j].Node
		}
		idi := strings.TrimPrefix(table[i].ID, "hb#")
		idj := strings.TrimPrefix(table[j].ID, "hb#")
		if idi != idj {
			return idi < idj
		}
		return table[i].Peer < table[j].Peer
	})
	output.Renderer{
		DefaultOutput: "tab=RUNNING:.state_icon,BEATING:.beating_icon,ID:.id,NODE:.node,PEER:.peer,TYPE:.type,DESC:.desc,CHANGED_AT:.changed_at",
		Output:        t.Output,
		Color:         t.Color,
		Data:          table,
		Colorize:      rawconfig.Colorize,
	}.Print()

	return nil
}

// NewCmdDaemonHeartbeatStatus is the name the list command answered to
// before the listings were named after what they render: a row per stream.
// It is kept for the readers whose fingers and scripts type it.
func NewCmdDaemonHeartbeatStatus(defaultNodeSelectorFilter string) *cobra.Command {
	cmd := NewCmdDaemonHeartbeatList(defaultNodeSelectorFilter)
	cmd.Use = "status"
	cmd.Aliases = nil
	cmd.Hidden = true
	return cmd
}
