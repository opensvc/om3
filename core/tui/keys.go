package tui

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/sizeconv"
	"github.com/rivo/tview"
)

var (
	deleteKeyMessage = "DeleteKeyMessage"
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
		switch event.Rune() {
		case '-':
			t.confirmAction(func() {
				t.deleteKey()
			}, deleteKeyMessage)
		case '+':
			t.createAddInputBox()
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
	resp, err := t.client.GetObjectDataKeysWithResponse(context.Background(), t.viewPath.Namespace, t.viewPath.Kind, t.viewPath.Name)
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
		t.keys.SetCell(row, 0, tview.NewTableCell(key.Name).SetSelectable(true))
		t.keys.SetCell(row, 1, tview.NewTableCell(sizeconv.BSizeCompact(float64(key.Size))).SetSelectable(false))
	}
}

func (t *App) updateKeyTextView() {
	if t.viewPath.IsZero() {
		return
	}
	if t.viewKey == "" {
		return
	}
	if t.skipIfConfigNotUpdated() {
		return
	}
	resp, err := t.client.GetObjectDataKeyWithResponse(context.Background(), t.viewPath.Namespace, t.viewPath.Kind, t.viewPath.Name, &api.GetObjectDataKeyParams{
		Name: t.viewKey,
	})
	if err != nil {
		t.errorf("%s", err)
		return
	}
	if resp.StatusCode() != http.StatusOK {
		t.errorf("status code: %s", resp.Status())
		return
	}

	t.initTextView()
	text := string(resp.Body)
	title := fmt.Sprintf("%s key %s", t.viewPath, t.viewKey)
	t.textView.SetTitle(title)
	t.textView.Clear()
	fmt.Fprint(t.textView, text)
}

func (t *App) deleteKey() {
	key := t.viewKey
	if key == "" {
		return
	}
	if t.viewPath.IsZero() || t.viewPath.Name == "" {
		return
	}
	path := t.viewPath
	c, err := client.New()
	if err != nil {
		t.errorf("failed to create client: %v", err)
		return
	}
	params := api.DeleteObjectDataKeyParams{
		Name: key,
	}
	resp, err := c.DeleteObjectDataKeyWithResponse(context.Background(), path.Namespace, path.Kind, path.Name, &params)
	if err != nil {
		t.errorf("failed to delete key %s: %v", key, err)
		return
	}
	switch resp.StatusCode() {
	case http.StatusNoContent:
		t.viewKey = ""
		row, col := t.keys.GetSelection()
		if t.keys.GetCell(row-1, col) != nil {
			t.keys.Select(row-1, col)
			t.viewKey = t.keys.GetCell(row-1, col).Text
		} else if t.keys.GetCell(row+1, col) != nil {
			t.keys.Select(row+1, col)
			t.viewKey = t.keys.GetCell(row+1, col).Text
		} else {
			t.keys.Select(0, 0)
		}
		return
	case http.StatusBadRequest:
		t.errorf("%s: %s", path, *resp.JSON400)
	case http.StatusUnauthorized:
		t.errorf("%s: %s", path, *resp.JSON401)
	case http.StatusForbidden:
		t.errorf("%s: %s", path, *resp.JSON403)
	case http.StatusInternalServerError:
		t.errorf("%s: %s", path, *resp.JSON500)
	default:
		t.errorf("%s: unexpected response: %s", path, resp.Status())
	}
}

func (t *App) createAddInputBox() {
	if t.viewPath.IsZero() {
		return
	}
	t.focused = true
	paddingSpace := func() string {
		return strings.Repeat(" ", 3)
	}

	grid := tview.NewGrid().
		SetRows(12, 0, 0).
		SetColumns(0, 45, 0)

	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.SetTitle(paddingSpace() + "Add key to " + t.viewPath.String() + paddingSpace()).SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	grid.AddItem(flex, 0, 1, 1, 1, 0, 0, true)

	clean := func() {
		t.focused = false
		t.flex.RemoveItem(grid)
		t.app.SetFocus(t.flex.GetItem(1))
	}

	errorClean := func(format string, args ...any) {
		t.errorf(format, args...)
		clean()
	}

	inputKeyName := tview.NewInputField().SetLabel("Key name:").SetFieldWidth(20).
		SetFieldBackgroundColor(tcell.ColorDarkGray)

	inputFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	inputFlex.AddItem(nil, 0, 1, false)
	inputFlex.AddItem(inputKeyName, 30, 0, true)
	inputFlex.AddItem(nil, 0, 1, false)

	addKey := func() {
		keyName := strings.TrimSpace(inputKeyName.GetText())
		if keyName == "" {
			return
		}
		c, err := client.New()
		if err != nil {
			errorClean("failed to create client: %v", err)
			return
		}
		path := t.viewPath
		param := api.PostObjectDataKeyParams{
			Name: keyName,
		}
		response, err := c.PostObjectDataKeyWithBodyWithResponse(context.Background(), path.Namespace, path.Kind, path.Name, &param, "", nil)
		if err != nil {
			errorClean("%s: %v", path, err)
			return
		}
		switch {
		case response.StatusCode() == http.StatusNoContent:
			clean()
			return
		case response.StatusCode() == http.StatusConflict:
			errorClean("%s: key already exists.", path)
			return
		case response.JSON400 != nil:
			errorClean("%s: %s", path, *response.JSON400)
			return
		case response.JSON401 != nil:
			errorClean("%s: %s", path, *response.JSON401)
			return
		case response.JSON403 != nil:
			errorClean("%s: %s", path, *response.JSON403)
			return
		case response.JSON413 != nil:
			errorClean("%s: %s", path, *response.JSON413)
			return
		case response.JSON500 != nil:
			errorClean("%s: %s", path, *response.JSON500)
			return
		default:
			errorClean("%s: unexpected response: %s", path, response.Status())
			return
		}
	}

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyESC:
			clean()
			return nil
		case tcell.KeyEnter:
			addKey()
			return nil
		}
		return event
	})

	flex.AddItem(nil, 0, 1, false)
	flex.AddItem(inputFlex, 1, 0, true)
	flex.AddItem(nil, 0, 1, false)

	t.flex.AddItem(grid, 0, 1, true)
	t.app.SetFocus(flex)
}
