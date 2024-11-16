package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/opensvc/om3/core/colorstatus"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/rivo/tview"
)

func (t *App) updateInstanceView() {
	if t.viewPath.IsZero() {
		return
	}
	if t.viewNode == "" {
		return
	}
	if t.skipIfInstanceNotUpdated() {
		return
	}
	digest := t.Frame.Current.GetObjectStatus(t.viewPath)

	title := fmt.Sprintf("%s@%s status", t.viewPath, t.viewNode)

	table := tview.NewTable()
	table.SetTitle(title)
	table.SetBorder(false)
	table.SetEvaluateAllRows(true)

	table.SetSelectionChangedFunc(func(row, col int) {
		t.viewRID = ""
		if row == 0 {
			return
		}
		if col == 0 {
			t.viewRID = table.GetCell(row, col).Text
		}

	})

	selectedFunc := func(row, col int) {
		cell := table.GetCell(row, col)
		rid := table.GetCell(row, 0).Text
		var selected *bool
		switch {
		case row == 0:
		case col == 0:
			v := t.toggleRID(t.viewPath.String(), t.viewNode, rid)
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
		for i := 1; i < table.GetRowCount(); i++ {
			selectedFunc(i, 0)
		}
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
			table.SetSelectable(true, true)
		case tcell.KeyCtrlA:
			selectAll()
		case tcell.KeyEnter:
			//onEnter(event)
			return nil // prevents the default select behaviour
		}
		switch event.Rune() {
		case ' ':
			setSelection(table)
		}
		return event
	})

	cellResourceId := func(rid string) *tview.TableCell {
		cell := tview.NewTableCell(rid).SetAttributes(tcell.AttrBold).SetSelectable(true)
		if t.isResourceSelected(t.viewPath.String(), t.viewNode, rid) {
			cell.SetBackgroundColor(colorSelected)
		}
		return cell
	}
	cellLog := func(entry *resource.StatusLogEntry) *tview.TableCell {
		s := entry.String()
		switch entry.Level {
		case "error":
			s = tview.TranslateANSI(rawconfig.Colorize.Error(s))
		case "warn":
			s = tview.TranslateANSI(rawconfig.Colorize.Warning(s))
		}
		return tview.NewTableCell(s).SetSelectable(false)
	}
	cellFlags := func(resourceStatus resource.Status, instanceState instance.States) *tview.TableCell {
		s := instanceState.Status.ResourceFlagsString(*resourceStatus.ResourceID, resourceStatus)
		s += instanceState.Monitor.ResourceFlagRestartString(*resourceStatus.ResourceID, resourceStatus)
		s = tview.TranslateANSI(s)
		return tview.NewTableCell(s).SetSelectable(false)
	}

	i := 0
	instanceState, ok := digest.Instances.ByNode()[t.viewNode]
	if !ok {
		goto end
	}

	table.SetCell(i, 0, tview.NewTableCell("RID").SetTextColor(colorTitle).SetSelectable(false))
	table.SetCell(i, 1, tview.NewTableCell("FLAGS").SetTextColor(colorTitle).SetSelectable(false))
	table.SetCell(i, 2, tview.NewTableCell("STATUS").SetTextColor(colorTitle).SetSelectable(false))
	table.SetCell(i, 3, tview.NewTableCell("LABEL").SetTextColor(colorTitle).SetSelectable(false))
	for _, resourceStatus := range instanceState.Status.SortedResources() {
		i += 1
		table.SetCell(i, 0, cellResourceId(resourceStatus.ResourceID.String()))
		table.SetCell(i, 1, cellFlags(resourceStatus, instanceState))
		table.SetCell(i, 2, tview.NewTableCell(tview.TranslateANSI(colorstatus.Sprint(resourceStatus.Status, rawconfig.Colorize))).SetSelectable(false))
		table.SetCell(i, 3, tview.NewTableCell(resourceStatus.Label).SetSelectable(false))
		for _, entry := range resourceStatus.Log {
			i += 1
			table.SetCell(i, 0, tview.NewTableCell("").SetSelectable(false))
			table.SetCell(i, 1, tview.NewTableCell("").SetSelectable(false))
			table.SetCell(i, 2, tview.NewTableCell("").SetSelectable(false))
			table.SetCell(i, 3, cellLog(entry))
		}
	}

end:
	t.flex.Clear()
	t.flex.AddItem(t.head, 1, 0, false)
	t.flex.AddItem(table, 0, 1, true)
	t.app.SetFocus(table)
}
