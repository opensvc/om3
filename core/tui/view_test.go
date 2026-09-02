package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// newTestApp returns an App laid out on a simulation screen, without running
// the event loop.
func newTestApp(t *testing.T) *App {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(120, 24)

	a := NewApp(nil)
	a.initHeadTextView()
	a.initObjectsTable()
	a.initErrsTextView()
	a.app = tview.NewApplication().SetScreen(screen)
	a.flex = tview.NewFlex().SetDirection(tview.FlexRow)
	a.app.SetRoot(a.flex, true)
	a.mount(a.objects)
	return a
}

// stubViews replaces the view registry with views recording their enter and
// leave calls, and restores the real registry when the test ends.
func stubViews(t *testing.T, log *[]string, ids ...viewId) {
	t.Helper()
	saved := viewDefs
	t.Cleanup(func() { viewDefs = saved })

	viewDefs = make(map[viewId]viewDef, len(ids))
	for _, id := range ids {
		name := saved[id].title
		viewDefs[id] = viewDef{
			title: name,
			enter: func(*App) { *log = append(*log, "enter "+name) },
			leave: func(*App) { *log = append(*log, "leave "+name) },
		}
	}
}

// Every view id must be declared in the registry: viewId.String() and the
// enter, leave and refresh dispatches all go through it.
func TestViewDefsAreComplete(t *testing.T) {
	for id := viewObject; id < viewLast; id++ {
		def, ok := viewDefs[id]
		if !ok {
			t.Errorf("view id %d has no viewDefs entry", int(id))
			continue
		}
		if def.title == "" {
			t.Errorf("view id %d has no title", int(id))
		}
		if def.enter == nil {
			t.Errorf("view %s has no enter hook", def.title)
		}
	}
}

func TestNavStack(t *testing.T) {
	var log []string
	stubViews(t, &log, viewObject, viewPool, viewPoolVolume, viewLog)
	a := newTestApp(t)

	if !a.atRoot() || a.focus() != viewObject {
		t.Fatalf("a new app must be rooted on the object view, got %s", a.stack)
	}

	a.nav(viewLog)
	if a.focus() != viewLog || len(a.stack) != 2 {
		t.Fatalf("nav to the log view: got %s", a.stack)
	}

	// hitting the log key again must not stack a second log frame
	a.nav(viewLog)
	if len(a.stack) != 2 {
		t.Fatalf("nav to the focused view must be a no-op, got %s", a.stack)
	}

	a.back()
	if a.focus() != viewObject || !a.atRoot() {
		t.Fatalf("back to the object view: got %s", a.stack)
	}

	if want := "leave objects enter log leave log enter objects"; join(log) != want {
		t.Fatalf("hooks: got %q, want %q", join(log), want)
	}
}

// Drilling down keeps one frame per drilled element, and coming back restores
// the element of the frame below.
func TestNavStackDrillDown(t *testing.T) {
	var log []string
	stubViews(t, &log, viewObject, viewPool, viewPoolVolume)
	a := newTestApp(t)

	a.nav(viewPool) // the pool list
	a.selectedElement = "pool1"
	a.nav(viewPool) // that pool, per node
	if len(a.stack) != 3 {
		t.Fatalf("drilling down into a pool must push a frame, got %s", a.stack)
	}

	// re-entering the same pool must not stack a second frame
	a.nav(viewPool)
	if len(a.stack) != 3 {
		t.Fatalf("re-entering the same pool must be a no-op, got %s", a.stack)
	}

	a.nav(viewPoolVolume)
	if a.focus() != viewPoolVolume || a.selectedElement != "pool1" {
		t.Fatalf("the volume view must inherit the drilled pool, got %q", a.selectedElement)
	}

	a.back()
	if a.focus() != viewPool || a.selectedElement != "pool1" {
		t.Fatalf("back to the pool detail: got %s %q", a.stack, a.selectedElement)
	}

	a.back()
	if a.focus() != viewPool || a.selectedElement != "" {
		t.Fatalf("back to the pool list must clear the drilled pool, got %s %q", a.stack, a.selectedElement)
	}

	a.back()
	if !a.atRoot() || a.focus() != viewObject {
		t.Fatalf("back to the object view: got %s", a.stack)
	}
}

// The cursor is remembered per frame, so coming back to a view lands where the
// user left it.
func TestNavStackKeepsCursorPerFrame(t *testing.T) {
	var log []string
	stubViews(t, &log, viewObject, viewInstance, viewLog)
	a := newTestApp(t)

	a.nav(viewInstance)
	a.frame().position = Position{row: 12, col: 3}

	a.nav(viewLog)
	if got := a.frame().position; got != (Position{}) {
		t.Fatalf("a new frame must start at the first cell, got %v", got)
	}

	a.back()
	if got := a.frame().position; got != (Position{row: 12, col: 3}) {
		t.Fatalf("back must restore the frame cursor, got %v", got)
	}
}

// At the root of the stack there is nothing to pop.
func TestBackAtRoot(t *testing.T) {
	var log []string
	stubViews(t, &log, viewObject)
	a := newTestApp(t)

	a.back()
	if !a.atRoot() {
		t.Fatalf("back at the root must not pop, got %s", a.stack)
	}
}

// navRoot drops the whole stack, leaving and entering as usual.
func TestNavRoot(t *testing.T) {
	var log []string
	stubViews(t, &log, viewObject, viewContext, viewRelay)
	a := newTestApp(t)

	a.nav(viewRelay)
	a.navRoot(viewContext)
	if !a.atRoot() || a.focus() != viewContext {
		t.Fatalf("navRoot must leave a single frame, got %s", a.stack)
	}
	if want := "leave objects enter relay leave relay enter context"; join(log) != want {
		t.Fatalf("hooks: got %q, want %q", join(log), want)
	}
}

// mount lays the head bar, the banners, the body and the errors bar out, and
// remount restores the whole layout, banners included.
func TestMountLayout(t *testing.T) {
	a := newTestApp(t)

	banner := tview.NewTable()
	body := tview.NewTable()
	a.mount(body, mountBanner{primitive: banner, height: 4})

	if got, want := a.flex.GetItemCount(), 4; got != want {
		t.Fatalf("mounted item count: got %d, want %d", got, want)
	}
	if a.flex.GetItem(0) != a.head || a.flex.GetItem(1) != banner ||
		a.flex.GetItem(2) != body || a.flex.GetItem(3) != a.errs {
		t.Fatal("mounted items are not head, banner, body, errs")
	}
	if a.body() != body {
		t.Fatal("body() must return the mounted body")
	}

	saved := a.mounted
	a.mount(tview.NewTable())
	a.remount(saved)
	if a.flex.GetItemCount() != 4 || a.flex.GetItem(1) != banner || a.body() != body {
		t.Fatal("remount must restore the banners and the body")
	}
}

func join(l []string) string {
	s := ""
	for i, e := range l {
		if i > 0 {
			s += " "
		}
		s += e
	}
	return s
}
