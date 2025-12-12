package tui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/opensvc/om3/v3/core/monitor"
	"github.com/opensvc/om3/v3/core/naming"
)

var (
	hbIndexRow   = -1
	separatorBar = "│"
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
		case (row >= hbIndexRow && row <= hbIndexRow+2) && (col >= t.headerRightCol && col <= t.firstInstanceCol+len(t.Current.Cluster.Config.Nodes)-1):
			if hbIndexRow == -1 {
				return
			}
			var nodeFilter string
			if col >= t.firstInstanceCol {
				nodeFilter = table.GetCell(0, col).Text
			}
			var hbType string
			if row > 8 {
				hbType = table.GetCell(row, 3).Text[3:]
			}
			t.hbFilter = HbStatusFilter{
				Name:     hbType,
				NodeName: nodeFilter,
			}
			t.nav(viewHbStatus)
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
			if t.focused {
				return event
			}
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

	nodesPopulate := func(row int, selectable bool, valueFn func(string) string) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(valueFn(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(selectable))
		}
	}

	nodesCells := func(row int, selectable bool) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			t.objects.SetCell(row, t.firstInstanceCol+i, t.cellNode(nodename, selectable))
		}
	}

	nodesScoreCells := func(row int) { nodesPopulate(row, false, func(n string) string { return t.StrNodeScore(n) }) }
	nodesLoadCells := func(row int) { nodesPopulate(row, false, func(n string) string { return t.StrNodeLoad(n) }) }
	nodesMemCells := func(row int) { nodesPopulate(row, false, func(n string) string { return t.StrNodeMem(n) }) }
	nodesSwapCells := func(row int) { nodesPopulate(row, false, func(n string) string { return t.StrNodeSwap(n) }) }
	nodesStateCells := func(row int) { nodesPopulate(row, false, func(n string) string { return t.StrNodeStates(n) }) }
	nodesHbCells := func(row int) { nodesPopulate(row, true, func(n string) string { return t.StrNodeHbMode(n) }) }
	nodesHb1Cells := func(row int, hbType string) {
		nodesPopulate(row, true, func(n string) string { return t.StrHeartbeat(n, hbType) })
	}
	nodesArbitratorCells := func(row int, arbitratorName string) {
		nodesPopulate(row, false, func(n string) string { return t.StrNodeArbitratorStatus(arbitratorName, n) })
	}
	nodesDaemonState := func(row int) { nodesPopulate(row, false, func(n string) string { return t.StrDaemonState(n) }) }
	nodesDaemonUptime := func(row int) { nodesPopulate(row, false, func(n string) string { return t.StrDaemonUptime(n) }) }
	nodesUptime := func(row int) { nodesPopulate(row, false, func(n string) string { return t.StrNodeUptime(n) }) }
	nodesVersion := func(row int) { nodesPopulate(row, false, func(n string) string { return t.StrNodeVersion(n) }) }

	fmtSubElement := func(name string) string {
		return separatorBar + name
	}

	objects := []HeaderObject{
		{
			Left:  t.newLeftHeaderCell("CLUSTER", t.Current.Cluster.Config.Name).withValueSelectable(),
			Right: t.newRightHeaderCell("NODE"),
			Populate: func(row int) {
				nodesCells(row, true)
			},
		},
		{
			Left:     t.newLeftHeaderCell("EVENT", fmt.Sprintf("%d", t.eventCount)).withValueSelectable(),
			Right:    t.newRightHeaderCell("UPTIME"),
			Populate: nodesUptime,
		},
		{
			Left:     t.newLeftHeaderCell("LAST", "0s"),
			Right:    t.newRightHeaderCell("SCORE"),
			Populate: nodesScoreCells,
		},
		{
			Left:     HeaderCell{},
			Right:    t.newRightHeaderCell(fmtSubElement("LOAD")),
			Populate: nodesLoadCells,
		},
		{
			Left:     HeaderCell{},
			Right:    t.newRightHeaderCell(fmtSubElement("MEM")),
			Populate: nodesMemCells,
		},
		{
			Left:     HeaderCell{},
			Right:    t.newRightHeaderCell(fmtSubElement("SWAP")),
			Populate: nodesSwapCells,
		},
		{
			Left:     HeaderCell{},
			Right:    t.newRightHeaderCell("DAEMON"),
			Populate: nodesDaemonState,
		},
		{
			Left:     HeaderCell{},
			Right:    t.newRightHeaderCell(fmtSubElement("UPTIME")),
			Populate: nodesDaemonUptime,
		},
	}

	if t.NodeVersions().Len() > 1 {
		objects = append(objects, HeaderObject{
			Left:     HeaderCell{},
			Right:    t.newRightHeaderCell("VERSION"),
			Populate: nodesVersion,
		})
	}

	if len(t.Current.Cluster.Config.Nodes) > 1 {
		hbIndexRow = len(objects)
		objects = append(objects, HeaderObject{
			Left:     HeaderCell{},
			Right:    t.newRightHeaderCell("HB").withTitleSelectable(),
			Populate: nodesHbCells,
		})

		for _, hbType := range []string{"rx", "tx"} {
			name := fmtSubElement(hbType)
			objects = append(objects, HeaderObject{
				Left:  HeaderCell{},
				Right: t.newRightHeaderCell(name).withTitleSelectable(),
				Populate: func(row int) {
					nodesHb1Cells(row, hbType)
				},
			})
		}
	}

	arbitratorNames := t.Current.ArbitratorNames()
	if len(arbitratorNames) > 0 {
		objects = append(objects, HeaderObject{
			Left:     HeaderCell{},
			Right:    t.newRightHeaderCell("ARBITRATORS"),
			Populate: func(row int) {},
		})

		for _, arbitratorName := range arbitratorNames {
			name := fmtSubElement(arbitratorName)
			objects = append(objects, HeaderObject{
				Left:  HeaderCell{},
				Right: t.newRightHeaderCell(name),
				Populate: func(row int) {
					nodesArbitratorCells(row, arbitratorName)
				},
			})
		}
	}

	objects = append(objects,
		HeaderObject{
			Left:     HeaderCell{},
			Right:    t.newRightHeaderCell("STATE"),
			Populate: nodesStateCells,
		},
		HeaderObject{
			Left: HeaderCell{},
			Right: HeaderCell{
				Value: "┼",
			},
			Populate: func(row int) {},
		})

	t.lastDraw = time.Now()

	t.objects.Clear()
	t.objects.SetTitle(fmt.Sprintf("%s objects", t.Frame.Selector))

	for i, obj := range objects {
		row := i
		t.objects.SetCell(row, 0, tview.NewTableCell(obj.Left.Title).SetTextColor(colorTitle).SetSelectable(obj.Left.TitleSelectable))
		t.objects.SetCell(row, 1, tview.NewTableCell(obj.Left.Value).SetSelectable(obj.Left.ValueSelectable))
		t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
		t.objects.SetCell(row, 3, tview.NewTableCell(obj.Right.Title).SetTextColor(colorTitle).SetSelectable(obj.Right.TitleSelectable))
		t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
		obj.Populate(row)
	}

	row := len(objects)
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

	row, col := t.objects.GetSelection()
	path := t.objects.GetCell(row, col).Text
	p, err := naming.ParsePath(path)
	if err != nil {
		return
	}
	t.viewPath = p
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

func (t *App) newLeftHeaderCell(title, value string) HeaderCell {
	return HeaderCell{
		Title: title,
		Value: value,
	}
}

func (t *App) newRightHeaderCell(title string) HeaderCell {
	return HeaderCell{
		Title: title,
		Value: separatorBar,
	}
}

func (h HeaderCell) withTitleSelectable() HeaderCell {
	h.TitleSelectable = true
	return h
}

func (h HeaderCell) withValueSelectable() HeaderCell {
	h.ValueSelectable = true
	return h
}
