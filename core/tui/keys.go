package tui

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gdamore/tcell/v2"
	"github.com/opensvc/om3/util/sizeconv"
	"github.com/rivo/tview"
)

func (t *App) initKeysTable() {
	table := tview.NewTable()
	table.SetBorder(false)

	onEnter := func(event *tcell.EventKey) {
		t.nav(viewKey)
	}

	table.SetSelectionChangedFunc(func(row, col int) {
		t.viewKey = ""
		if row == 0 {
			return
		}
		if col == 0 {
			t.viewKey = table.GetCell(row, col).Text
		}

	})
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
			table.SetSelectable(true, false)
		case tcell.KeyEnter:
			onEnter(event)
			return nil // prevents the default select behaviour
		}
		return event
	})
	t.keys = table
}

func (t *App) updateKeysView() {
	if t.viewPath.IsZero() {
		return
	}
	if t.skipIfConfigNotUpdated() {
		return
	}
	resp, err := t.client.GetObjectKVStoreKeysWithResponse(context.Background(), t.viewPath.Namespace, t.viewPath.Kind, t.viewPath.Name)
	if err != nil {
		return
	}
	if resp.StatusCode() != http.StatusOK {
		return
	}
	t.keys.Clear()
	t.keys.SetTitle(fmt.Sprintf("%s keys", t.viewPath))
	t.keys.SetCell(0, 0, tview.NewTableCell("NAME").SetTextColor(colorTitle).SetSelectable(false))
	t.keys.SetCell(0, 1, tview.NewTableCell("SIZE").SetTextColor(colorTitle).SetSelectable(false))
	for i, key := range resp.JSON200.Items {
		row := 1 + i
		t.keys.SetCell(row, 0, tview.NewTableCell(key.Key).SetSelectable(true))
		t.keys.SetCell(row, 1, tview.NewTableCell(sizeconv.BSizeCompact(float64(key.Size))).SetSelectable(false))
	}
}
