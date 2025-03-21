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
)

type (
	CmdDaemonHeartbeatStatus struct {
		OptsGlobal
		NodeSelector string
		Name         string
	}
)

func (t *CmdDaemonHeartbeatStatus) Run() error {
	if t.NodeSelector == "" {
		return fmt.Errorf("--node is empty")
	}
	cli, err := client.New()
	if err != nil {
		return err
	}
	getter := cli.NewGetDaemonStatus()
	b, err := getter.Get()
	if err != nil {
		return err
	}
	var data clusterdump.Data
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	nodeMap := make(map[string]any)
	if nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(cli)).Expand(); err != nil {
		return err
	} else {
		for _, nodename := range nodenames {
			nodeMap[nodename] = nil
		}
	}
	if t.Name != "" && !strings.HasPrefix(t.Name, "hb#") {
		t.Name = "hb#" + t.Name
	}
	table := make(daemonsubsystem.HeartbeatStreamPeerStatusTable, 0)
	for nodename, nodeData := range data.Cluster.Node {
		if _, ok := nodeMap[nodename]; !ok {
			continue
		}
		for _, e := range nodeData.Daemon.Heartbeat.Table(nodename) {
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
