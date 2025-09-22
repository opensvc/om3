package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/opensvc/om3/core/monitor"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/rivo/tview"
)

func (t *App) initObjectsTable() {
	table := tview.NewTable()
	table.SetEvaluateAllRows(true)

	onEnter := func(event *tcell.EventKey) {
		row, col := table.GetSelection()
		switch {
		case !t.viewPath.IsZero() && t.viewNode != "":
			t.initTextView()
			t.nav(viewInstance)
		case t.viewPath.Kind == naming.KindCfg || t.viewPath.Kind == naming.KindSec:
			t.nav(viewKeys)
		case row == 0 && col == 1:
			t.listContexts()
		case row == 1 && col == 1:
			t.nav(viewEvents)
		}
	}

	selectedFunc := func(row, col int) {
		cell := table.GetCell(row, col)
		path := table.GetCell(row, 0).Text
		node := table.GetCell(0, col).Text
		var selected *bool
		switch {
		case row == 0 && col >= t.firstInstanceCol:
			v := t.toggleNode(node)
			selected = &v
		case row < t.firstObjectRow:
		case col == 0:
			v := t.togglePath(path)
			selected = &v
		case col >= t.firstInstanceCol:
			v := t.toggleInstance(path, node)
			selected = &v
		}
		if selected != nil && *selected {
			cell.SetBackgroundColor(colorSelected)
		} else {
			cell.SetBackgroundColor(colorNone)
		}
	}

	table.SetSelectedFunc(selectedFunc)

	setSelection := func(table *tview.Table) {
		row, col := table.GetSelection()
		cell := table.GetCell(row, col)
		cell.SetBackgroundColor(colorSelected)
		table.SetCell(row, col, cell)
		selectedFunc(row, col)
	}

	selectAll := func() {
		for i := t.firstObjectRow; i < table.GetRowCount(); i++ {
			selectedFunc(i, 0)
		}
	}

	handleCursorPosition := func(row, column int) {
		cell := table.GetCell(row, column)
		if cell.NotSelectable {
			for i := 0; i < table.GetRowCount(); i++ {
				if i == row {
					continue
				}
				c := table.GetCell(i, column)
				if !c.NotSelectable {
					table.Select(i, column)
					return
				}
			}
		}
	}

	table.SetSelectionChangedFunc(func(row, col int) {
		t.viewNode = ""
		t.viewPath = naming.Path{}
		if row >= t.firstObjectRow {
			path := t.objects.GetCell(row, 0).Text
			p, err := naming.ParsePath(path)
			if err != nil {
				return
			}
			t.viewPath = p
		}
		if col >= t.firstInstanceCol {
			t.viewNode = t.objects.GetCell(0, col).Text
		}
		t.position = Position{row: row, col: col}
		handleCursorPosition(row, col)
	})
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
			table.SetSelectable(true, true)
		case tcell.KeyESC:
			t.resetSelectedNodes()
			t.resetSelectedPaths()
			t.resetSelectedInstances()
		case tcell.KeyCtrlA:
			selectAll()
		case tcell.KeyEnter:
			onEnter(event)
			return nil // prevents the default select behaviour
		}
		switch event.Rune() {
		case ' ':
			setSelection(table)
		}
		return event
	})
	t.objects = table
}

func (t *App) updateObjects() {
	nodesCells := func(row int, selectable bool) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			t.objects.SetCell(row, t.firstInstanceCol+i, t.cellNode(nodename, selectable))
		}
	}

	nodesScoreCells := func(row int) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeScore(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	nodesLoadCells := func(row int) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeLoad(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	nodesMemCells := func(row int) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeMem(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	nodesSwapCells := func(row int) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeSwap(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	nodesStateCells := func(row int) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeStates(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	nodesHbCells := func(row int) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeHbMode(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	nodesHb1Cells := func(row int, stream daemonsubsystem.HeartbeatStream) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeHbStatus(stream, nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	nodesArbitratorCells := func(row int, arbitratorName string) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeArbitratorStatus(arbitratorName, nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	t.lastDraw = time.Now()

	t.objects.Clear()
	t.objects.SetTitle(fmt.Sprintf("%s objects", t.Frame.Selector))

	row := 0
	t.objects.SetCell(row, 0, tview.NewTableCell("CLUSTER").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell(t.Current.Cluster.Config.Name).SetSelectable(true))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("NODE").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesCells(row, true)

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("EVENT").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", t.eventCount)).SetSelectable(true))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("SCORE").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesScoreCells(row)

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("LAST").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("0s").SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("│LOAD").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesLoadCells(row)

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("│MEM").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesMemCells(row)

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("│SWAP").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesSwapCells(row)

	if len(t.Current.Cluster.Config.Nodes) > 1 {
		row++
		t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
		t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
		t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
		t.objects.SetCell(row, 3, tview.NewTableCell("HB").SetTextColor(colorTitle).SetSelectable(false))
		t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
		nodesHbCells(row)

		for _, hbStatus := range t.Current.Cluster.Node[t.Frame.Nodename].Daemon.Heartbeat.Streams {
			name := "│" + strings.TrimPrefix(hbStatus.ID, "hb#") + tview.TranslateANSI(monitor.StrThreadAlerts(hbStatus.Alerts))
			row++
			t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
			t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
			t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
			t.objects.SetCell(row, 3, tview.NewTableCell(name).SetTextColor(colorTitle).SetSelectable(false))
			t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
			nodesHb1Cells(row, hbStatus)
		}
	}

	arbitratorNames := t.Current.ArbitratorNames()
	if len(arbitratorNames) > 0 {
		row++
		t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
		t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
		t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
		t.objects.SetCell(row, 3, tview.NewTableCell("ARBITRATORS").SetTextColor(colorTitle).SetSelectable(false))
		t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))

		for _, arbitratorName := range arbitratorNames {
			name := "│" + arbitratorName
			row++
			t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
			t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
			t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
			t.objects.SetCell(row, 3, tview.NewTableCell(name).SetTextColor(colorTitle).SetSelectable(false))
			t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
			nodesArbitratorCells(row, arbitratorName)
		}
	}

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("STATE").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesStateCells(row)

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("┼").SetTextColor(colorTitle).SetSelectable(false))

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("PATH").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("AVAIL").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("ORCHESTRATE").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("UP").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesCells(row, false)

	t.firstObjectRow = row + 1

	t.objects.SetFixed(t.firstObjectRow, 2)

	for _, path := range t.paths() {
		row++
		t.objects.SetCell(row, 0, t.cellObjectPath(path))
		t.objects.SetCell(row, 1, t.cellObjectStatus(path))
		t.objects.SetCell(row, 2, t.cellObjectOrchestrate(path))
		t.objects.SetCell(row, 3, t.cellObjectRunning(path))
		t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
		for j, nodename := range t.Current.Cluster.Config.Nodes {
			t.objects.SetCell(row, 5+j, t.cellInstanceStatus(path, nodename))
		}
	}
}

func (t *App) cellObjectOrchestrate(path string) *tview.TableCell {
	var s string
	if objectStatus := t.Current.Cluster.Object[path]; objectStatus.ActorStatus != nil {
		s = objectStatus.Orchestrate
	}
	return tview.NewTableCell(s).SetSelectable(false)
}

func (t *App) cellObjectRunning(path string) *tview.TableCell {
	s := tview.TranslateANSI(t.StrObjectRunning(path))
	return tview.NewTableCell(s).SetSelectable(false)
}

func (t *App) cellObjectStatus(path string) *tview.TableCell {
	s := tview.TranslateANSI(monitor.StrObjectStatus(t.Current.Cluster.Object[path]))
	return tview.NewTableCell(s).SetSelectable(false)
}

func (t *App) cellInstanceStatus(path, node string) *tview.TableCell {
	s := tview.TranslateANSI(t.StrObjectInstance(path, node, t.Current.Cluster.Object[path].Scope))
	cell := tview.NewTableCell(s)
	if t.isInstanceSelected(path, node) {
		cell.SetBackgroundColor(colorSelected)
	}
	return cell
}

func (t *App) cellNode(node string, selectable bool) *tview.TableCell {
	cell := tview.NewTableCell(node).SetAttributes(tcell.AttrBold).SetSelectable(selectable)
	if selectable && t.isNodeSelected(node) {
		cell.SetBackgroundColor(colorSelected)
	}
	return cell
}

func (t *App) cellObjectPath(path string) *tview.TableCell {
	cell := tview.NewTableCell(path).SetAttributes(tcell.AttrBold)
	if t.isPathSelected(path) {
		cell.SetBackgroundColor(colorSelected)
	}
	return cell
}
