package tui

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/opensvc/om3/v3/core/client"
)

// TestRuneCOnAnInstanceCellShowsTheObjectConfig drives the real
// application against this node's daemon and presses 'c' on a cell that
// names both an object and a node.
//
// That cell used to answer with the node's configuration, because the
// config view tested viewNode before viewPath while 'e' tested them the
// other way round. The unit test on configTargetFor covers the decision;
// this covers the path from the keypress to the title on screen.
func TestRuneCOnAnInstanceCellShowsTheObjectConfig(t *testing.T) {
	cli, err := client.New()
	if err != nil {
		t.Skipf("no client: %s", err)
	}
	if resp, err := cli.GetAuthWhoAmIWithResponse(context.Background()); err != nil {
		t.Skipf("no daemon: %s", err)
	} else if resp.StatusCode() != http.StatusOK {
		t.Skipf("no daemon: %s", resp.Status())
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(200, 40)

	a := NewApp(nil)
	if err := a.init(); err != nil {
		t.Fatal(err)
	}
	a.app.SetScreen(screen)
	go a.runEventReader()
	a.initContext()

	done := make(chan struct{})
	go func() { defer close(done); _ = a.app.Run() }()
	defer func() {
		a.stop()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the application did not stop")
		}
	}()

	time.Sleep(2 * time.Second)

	sync := func(f func()) {
		ch := make(chan struct{})
		a.app.QueueUpdateDraw(func() { f(); close(ch) })
		select {
		case <-ch:
		case <-time.After(5 * time.Second):
			t.Fatal("the event loop is wedged")
		}
	}

	// Land on an instance cell: a row that names an object, a column that
	// names a node. That is the selection the bug was about.
	var wantPath, wantNode string
	sync(func() {
		row := a.firstObjectRow
		col := a.firstInstanceCol
		if a.objects.GetRowCount() <= row || a.objects.GetColumnCount() <= col {
			return
		}
		a.objects.Select(row, col)
	})
	sync(func() {
		wantPath, wantNode = a.viewPath.String(), a.viewNode
	})
	if wantPath == "" || wantPath == "." || wantNode == "" {
		t.Skipf("no instance cell to land on: path=%q node=%q", wantPath, wantNode)
	}
	t.Logf("selected instance cell: path=%s node=%s", wantPath, wantNode)

	screen.InjectKey(tcell.KeyRune, 'c', tcell.ModNone)
	sync(func() {})
	time.Sleep(500 * time.Millisecond)

	var focus viewId
	var title string
	sync(func() {
		focus = a.focus()
		title = a.textView.GetTitle()
	})
	if focus != viewConfig {
		t.Fatalf("'c' left the focus on %s, wanted %s", focus, viewConfig)
	}
	if want := fmt.Sprintf("%s configuration", wantPath); title != want {
		t.Errorf("'c' on an instance cell titled the view %q, wanted %q", title, want)
	}
	if strings.HasPrefix(title, wantNode+" ") {
		t.Errorf("'c' showed the node configuration (%q) for a cell naming object %s", title, wantPath)
	}
}
