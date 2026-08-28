package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// A table declaring no selectable column must not lock the event loop up on
// page-down / page-up.
func TestCreateTableNoSelectableColumnPaging(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyPgDn, tcell.KeyPgUp} {
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatal(err)
		}
		screen.SetSize(120, 10)

		app := NewApp(nil)
		app.app = tview.NewApplication().SetScreen(screen)
		app.initHeadTextView()
		app.initErrsTextView()
		app.flex = tview.NewFlex().SetDirection(tview.FlexRow)
		app.flex.AddItem(app.head, 1, 0, false)
		app.app.SetRoot(app.flex, true)

		titles := []string{"RUNNING", "BEATING", "ID", "NODE", "PEER", "TYPE", "DESC", "CHANGED_AT"}
		elements := make([][]string, 0, 40)
		for r := 0; r < 40; r++ {
			row := make([]string, len(titles))
			for c := range titles {
				row[c] = fmt.Sprintf("r%dc%d", r, c)
			}
			elements = append(elements, row)
		}
		app.createTable(CreateTableOptions{
			title:             "heartbeats",
			titles:            titles,
			elementsList:      elements,
			selectableColumns: []int{},
		})

		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = app.app.Run()
		}()
		time.Sleep(200 * time.Millisecond)
		screen.InjectKey(key, 0, tcell.ModNone)
		time.Sleep(200 * time.Millisecond)
		app.app.Stop()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatalf("event loop locked up on key %v", tcell.KeyNames[key])
		}
	}
}
