package tui

// nodeIssuesRows returns the rows of the node issues view: one per issue
// of the node the view is about, or of every node when it is about none.
func (t *App) nodeIssuesRows() [][]string {
	nodenames := t.Current.Cluster.Config.Nodes
	if t.viewNodeIssues != "" {
		nodenames = []string{t.viewNodeIssues}
	}
	rows := make([][]string, 0)
	for _, nodename := range nodenames {
		for _, issue := range t.nodeIssues(nodename) {
			rows = append(rows, []string{nodename, issue})
		}
	}
	return rows
}

// updateNodeIssues lists the configuration faults a node reports about
// itself, which is what the mark on its states line is about.
//
// They are read from the node configuration the daemon distributes, so
// this says the same as om mon and om node config get, without asking
// the node.
func (t *App) updateNodeIssues() {
	t.createTable(CreateTableOptions{
		title:             "node issues",
		titles:            []string{"NODE", "ISSUE"},
		elementsList:      t.nodeIssuesRows(),
		selectableColumns: []int{},
	})
}
