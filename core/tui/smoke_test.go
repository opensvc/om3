package tui

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/opensvc/om3/v3/core/client"
)

// TestSmokeLiveDaemon drives the real application against the daemon of the
// node it runs on: it enters every view, pages through it and comes back to
// the object view. Skipped when no daemon answers.
func TestSmokeLiveDaemon(t *testing.T) {
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
	screen.SetSize(160, 12) // short on purpose: more hb lines than term lines

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

	readScreen := func() string {
		var b strings.Builder
		w, h := screen.Size()
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, _, _, _ := screen.GetContent(x, y)
				b.WriteRune(r)
			}
			b.WriteString("\n")
		}
		return b.String()
	}

	// let the first cluster data land
	time.Sleep(2 * time.Second)

	var dump func() string
	sync := func(f func()) {
		ch := make(chan struct{})
		a.app.QueueUpdateDraw(func() { f(); close(ch) })
		select {
		case <-ch:
		case <-time.After(5 * time.Second):
			t.Fatal("the event loop is wedged")
		}
	}
	// read the screen from the application loop, so the dump never catches a
	// repaint halfway through
	dump = func() string {
		var s string
		sync(func() { s = readScreen() })
		return s
	}
	// state is a snapshot of the application state, read on the tview loop:
	// nav() and the table cursor callbacks own it from there.
	type state struct {
		focus     viewId
		stack     string
		depth     int
		element   string
		flexItems int
	}
	inspect := func() state {
		var st state
		sync(func() {
			st = state{
				focus:     a.focus(),
				stack:     a.stack.String(),
				depth:     len(a.stack),
				element:   a.selectedElement,
				flexItems: a.flex.GetItemCount(),
			}
		})
		return st
	}
	sync(func() {
		t.Logf("nodes=%d objects=%d", len(a.Current.Cluster.Config.Nodes), len(a.Current.Cluster.Object))
	})
	key := func(k tcell.Key) {
		screen.InjectKey(k, 0, tcell.ModNone)
		sync(func() {})
	}

	for _, v := range []viewId{viewHbStatus, viewPool, viewNetwork, viewRelay, viewEvents, viewConfig, viewLog} {
		sync(func() { a.nav(v) })
		time.Sleep(300 * time.Millisecond)
		if st := inspect(); st.focus != v {
			t.Fatalf("nav to %s: focused on %s", v, st.focus)
		}
		for i := 0; i < 3; i++ {
			key(tcell.KeyPgDn)
		}
		key(tcell.KeyPgUp)
		key(tcell.KeyDown)

		st := inspect()
		if st.flexItems < 3 {
			t.Errorf("%s: only %d flex items", v, st.flexItems)
		}
		head := strings.TrimSpace(strings.SplitN(dump(), "\n", 2)[0])
		t.Logf("%-18s head=%q stack=%s", v, head, st.stack)
		if head == "" {
			t.Errorf("%s: the head bar is empty", v)
		}

		sync(func() { a.back() })
		time.Sleep(200 * time.Millisecond)
		if st := inspect(); st.depth != 1 || st.focus != viewObject {
			t.Fatalf("back from %s: stack is %s", v, st.stack)
		}
	}

	// drill down into the first pool, and back out
	sync(func() { a.nav(viewPool) })
	time.Sleep(300 * time.Millisecond)
	var (
		poolName string
		rows     int
		isTable  bool
	)
	sync(func() {
		table, ok := a.body().(*tview.Table)
		if !ok {
			return
		}
		isTable = true
		if rows = table.GetRowCount(); rows < 2 {
			return
		}
		table.Select(1, 0)
		poolName = table.GetCell(1, 0).Text
	})
	if !isTable {
		t.Fatal("the pool view body is not a table")
	}
	if rows < 2 {
		t.Skip("no pool to drill down into")
	}
	key(tcell.KeyEnter)
	time.Sleep(300 * time.Millisecond)
	if st := inspect(); st.focus != viewPool || st.depth != 3 || st.element != poolName {
		t.Fatalf("drilling into pool %q: stack=%s element=%q", poolName, st.stack, st.element)
	}
	t.Logf("drilled into pool %q, head=%q", poolName, strings.TrimSpace(strings.SplitN(dump(), "\n", 2)[0]))

	sync(func() { a.back() })
	time.Sleep(300 * time.Millisecond)
	if st := inspect(); st.focus != viewPool || st.depth != 2 || st.element != "" {
		t.Fatalf("back to the pool list: stack=%s element=%q", st.stack, st.element)
	}
	sync(func() { a.back() })
	if st := inspect(); st.depth != 1 {
		t.Fatalf("back to the object view: stack=%s", st.stack)
	}

	t.Logf("final screen:\n%s", dump())
}
