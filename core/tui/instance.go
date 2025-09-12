package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/opensvc/om3/core/colorstatus"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/rivo/tview"
)

func formatRel(data map[string]status.T) string {
	l := make([]string, len(data))
	i := 0
	for name, avail := range data {
		l[i] = name + " " + formatStatus(avail)
		i++
	}
	return strings.Join(l, ", ")
}

func formatInstance(data instance.Status) string {
	return formatInstanceAvail(data) + " " + formatInstanceIssues(data)
}

func formatInstanceAvail(data instance.Status) string {
	s := formatStatus(data.Avail)
	if data.Overall == status.Warn {
		s += " ⚠️"
	}
	return s + " (" + formatRecentTime(data.UpdatedAt) + ")"
}

func formatInstanceIssues(data instance.Status) string {
	var l []string
	if issue := formatInstanceProvisioned(data); issue != "" {
		l = append(l, issue)
	}
	if len(l) > 0 {
		return "⚠️ " + strings.Join(l, ", ")
	}
	return ""
}

func formatInstanceProvisioned(data instance.Status) string {
	switch data.Provisioned {
	case provisioned.False:
		return "not-provisioned"
	case provisioned.Mixed:
		return "mixed-provisioned"
	default:
		return ""
	}
}

func formatObjectIssues(data object.Status) string {
	var l []string
	if issue := formatObjectPlacement(data); issue != "" {
		l = append(l, issue)
	}
	if issue := formatObjectProvisioned(data); issue != "" {
		l = append(l, issue)
	}
	if len(l) > 0 {
		return "⚠️ " + strings.Join(l, ", ")
	}
	return ""
}

func formatObjectProvisioned(data object.Status) string {
	switch data.Provisioned {
	case provisioned.False:
		return "not-provisioned"
	case provisioned.Mixed:
		return "mixed-provisioned"
	default:
		return ""
	}
}

func formatObjectPlacement(data object.Status) string {
	switch data.PlacementState {
	case placement.NonOptimal:
		return "non-optimal placement"
	default:
		return ""
	}
}

func formatObjectAvail(data object.Status) string {
	return formatStatus(data.Avail) + " (" + formatRecentTime(data.UpdatedAt) + ")"
}

func formatObject(data object.Status) string {
	return formatObjectAvail(data) + " " + formatObjectIssues(data)
}

func formatStatus(data status.T) string {
	return tview.TranslateANSI(colorstatus.Sprint(data, rawconfig.Colorize))
}

func formatExpect(instanceMonitor instance.Monitor) string {
	return formatLocalExpect(instanceMonitor) + ", " + formatGlobalExpect(instanceMonitor)
}

func formatGlobalExpect(instanceMonitor instance.Monitor) string {
	s := "globally " + instanceMonitor.GlobalExpect.String()
	if !instanceMonitor.GlobalExpectUpdatedAt.IsZero() {
		s += fmt.Sprintf(" (%s)", formatRecentTime(instanceMonitor.GlobalExpectUpdatedAt))
	}
	return s
}

func formatLocalExpect(instanceMonitor instance.Monitor) string {
	s := "locally " + instanceMonitor.LocalExpect.String()
	if !instanceMonitor.LocalExpectUpdatedAt.IsZero() {
		s += fmt.Sprintf(" (%s)", formatRecentTime(instanceMonitor.LocalExpectUpdatedAt))
	}
	return s
}

func formatRecentTime(tm time.Time) string {
	age := time.Now().Sub(tm)
	switch {
	case age > time.Hour*48:
		day := time.Hour * 24
		i := int64(math.Round(float64(age / day)))
		return fmt.Sprintf("%dd ago", i)
	case age > time.Hour*2:
		i := int64(math.Round(float64(age / time.Hour)))
		return fmt.Sprintf("%dh ago", i)
	case age > time.Minute*2:
		i := int64(math.Round(float64(age / time.Minute)))
		return fmt.Sprintf("%dm ago", i)
	default:
		i := int64(math.Round(float64(age / time.Second)))
		return fmt.Sprintf("%ds ago", i)
	}
}

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

	table1 := tview.NewTable()
	table1.SetTitle(title)

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
	cellLog := func(entry resource.StatusLogEntry) *tview.TableCell {
		s := entry.String()
		switch entry.Level {
		case "error":
			s = tview.TranslateANSI(rawconfig.Colorize.Error(s))
		case "warn":
			s = tview.TranslateANSI(rawconfig.Colorize.Warning(s))
		}
		return tview.NewTableCell(s).SetSelectable(false)
	}
	cellFlags := func(rid string, resourceStatus resource.Status, instanceState instance.States) *tview.TableCell {
		s := instance.ResourceFlagsString(rid, instanceState.Monitor, instanceState.Status, resourceStatus)
		s = tview.TranslateANSI(s)
		return tview.NewTableCell(s).SetSelectable(false)
	}

	i := 0

	postamble := func() {
		t.flex.Clear()
		t.flex.AddItem(t.head, 1, 0, false)
		t.flex.AddItem(table1, i+2, 0, false)
		t.flex.AddItem(table, 0, 1, true)
		t.app.SetFocus(table)
	}

	instanceState, ok := digest.Instances.ByNode()[t.viewNode]
	if !ok {
		postamble()
	}

	setRow := func(prefix, rid string, resourceStatus resource.Status) {
		i += 1
		table.SetCell(i, 0, cellResourceId(prefix+rid))
		table.SetCell(i, 1, cellFlags(rid, resourceStatus, instanceState))
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

	table.SetCell(i, 0, tview.NewTableCell("RID").SetTextColor(colorTitle).SetSelectable(false))
	table.SetCell(i, 1, tview.NewTableCell("FLAGS").SetTextColor(colorTitle).SetSelectable(false))
	table.SetCell(i, 2, tview.NewTableCell("STATUS").SetTextColor(colorTitle).SetSelectable(false))
	table.SetCell(i, 3, tview.NewTableCell("LABEL").SetTextColor(colorTitle).SetSelectable(false))
	for _, resourceStatus := range instanceState.Status.SortedResources() {
		rid := resourceStatus.ResourceID.String()
		setRow("", rid, resourceStatus)
		if encapStatus, ok := instanceState.Status.Encap[rid]; ok {
			for rid, encapResourceStatus := range encapStatus.Resources {
				setRow(" ", rid, encapResourceStatus)
			}
		}
	}

	i = 0
	table1.SetCell(i, 0, tview.NewTableCell("OBJECT:").SetTextColor(colorTitle).SetSelectable(false))
	table1.SetCell(i, 1, tview.NewTableCell(formatObject(digest.Object)).SetSelectable(false))

	i++
	table1.SetCell(i, 0, tview.NewTableCell("INSTANCE:").SetTextColor(colorTitle).SetSelectable(false))
	table1.SetCell(i, 1, tview.NewTableCell(formatInstance(instanceState.Status)).SetSelectable(false))

	if instanceState.Status.IsFrozen() {
		i++
		table1.SetCell(i, 0, tview.NewTableCell(" FROZEN").SetTextColor(colorTitle).SetSelectable(false))
		table1.SetCell(i, 1, tview.NewTableCell(formatRecentTime(instanceState.Status.FrozenAt)).SetSelectable(false))
	}

	i++
	table1.SetCell(i, 0, tview.NewTableCell(" STATE").SetTextColor(colorTitle).SetSelectable(false))
	table1.SetCell(i, 1, tview.NewTableCell(instanceState.Monitor.State.String()).SetSelectable(false))

	i++
	table1.SetCell(i, 0, tview.NewTableCell(" EXPECT").SetTextColor(colorTitle).SetSelectable(false))
	table1.SetCell(i, 1, tview.NewTableCell(formatExpect(instanceState.Monitor)).SetSelectable(false))

	if len(instanceState.Monitor.Parents) > 0 {
		i++
		table1.SetCell(i, 0, tview.NewTableCell("START AFTER:").SetTextColor(colorTitle).SetSelectable(false))
		table1.SetCell(i, 1, tview.NewTableCell(formatRel(instanceState.Monitor.Parents)).SetSelectable(false))
	}

	if len(instanceState.Monitor.Children) > 0 {
		i++
		table1.SetCell(i, 0, tview.NewTableCell("STOP AFTER:").SetTextColor(colorTitle).SetSelectable(false))
		table1.SetCell(i, 1, tview.NewTableCell(formatRel(instanceState.Monitor.Children)).SetSelectable(false))
	}

	postamble()
}
