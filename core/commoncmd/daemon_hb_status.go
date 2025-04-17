package commoncmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/spf13/cobra"
)

type (
	CmdDaemonHeartbeatStatus struct {
		Color        string
		Output       string
		NodeSelector string
		PeerSelector string
		Name         string
	}
)

func NewCmdDaemonHeartbeatStatus(defaultNodeSelectorFilter string) *cobra.Command {
	options := CmdDaemonHeartbeatStatus{
		NodeSelector: defaultNodeSelectorFilter,
	}
	cmd := &cobra.Command{
		Use:   "status",
		Short: fmt.Sprintf("daemon heartbeat status"),
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

func (t *CmdDaemonHeartbeatStatus) Run() error {
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
	table := make(daemonsubsystem.HeartbeatStreamPeerStatusTable, 0)
	for nodename, nodeData := range data.Cluster.Node {
		if nodeMap != nil {
			if _, ok := nodeMap[nodename]; !ok {
				continue
			}
		}
		for _, e := range nodeData.Daemon.Heartbeat.Table(nodename) {
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
			table = append(table, e)
		}
	}
	output.Renderer{
		DefaultOutput: "tab=STATUS:.icon,ID:.id,NODE:.node,PEER:.peer,TYPE:.type,DESC:.desc,LAST_AT:.last_at",
		Output:        t.Output,
		Color:         t.Color,
		Data:          table,
		Colorize:      rawconfig.Colorize,
	}.Print()

	return nil
}
