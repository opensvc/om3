package tui

import (
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/daemon/daemonsubsystem"
)

func (t *App) updateHbStatus() {
	title := "heartbeats"
	titles := []string{"RUNNING", "BEATING", "ID", "NODE", "PEER", "TYPE", "DESC", "LAST_AT"}

	formatBool := func(b bool) string {
		if b {
			return "[green]O[white]"
		}
		return "[red]X[white]"
	}

	var elementsList [][]string

	isSingleNode := len(t.Current.Cluster.Node) == 1
	table := make(daemonsubsystem.HeartbeatStreamPeerStatusTable, 0)

	for nodeName, nodeData := range t.Current.Cluster.Node {
		for _, e := range nodeData.Daemon.Heartbeat.Table(nodeName, isSingleNode) {
			table = append(table, e)
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

	for _, e := range table {
		elementsList = append(elementsList, []string{
			formatBool(e.State == "running"),
			formatBool(e.IsBeating),
			e.ID,
			e.Node,
			e.Peer,
			e.Type,
			e.Desc,
			e.LastAt.Format(time.RFC3339),
		})
	}

	t.createTable(CreateTableOptions{
		title:             title,
		titles:            titles,
		elementsList:      elementsList,
		selectableColumns: []int{},
	})
}
