package tui

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// countingScreen counts the screen refreshes.
type countingScreen struct {
	tcell.SimulationScreen
	shows atomic.Int64
}

func (s *countingScreen) Show() {
	s.shows.Add(1)
	s.SimulationScreen.Show()
}

// The log and events views are written into by the goroutines streaming their
// content, which have no way to refresh the screen themselves: the text view
// changed handler has to do it for them. Run with -race: the writes must not
// race with the draw loop either.
func TestStreamedTextViewRedraws(t *testing.T) {
	for _, test := range []struct {
		view viewId
		name string
	}{
		{viewLog, "log"},
		{viewEvents, "events"},
	} {
		t.Run(test.name, func(t *testing.T) {
			sim := tcell.NewSimulationScreen("UTF-8")
			if err := sim.Init(); err != nil {
				t.Fatal(err)
			}
			sim.SetSize(120, 10)
			screen := &countingScreen{SimulationScreen: sim}

			a := NewApp(nil)
			a.initHeadTextView()
			a.initObjectsTable()
			a.initErrsTextView()
			a.app = tview.NewApplication().SetScreen(screen)
			a.flex = tview.NewFlex().SetDirection(tview.FlexRow)
			a.app.SetRoot(a.flex, true)
			a.mount(a.objects)

			done := make(chan struct{})
			go func() { defer close(done); _ = a.app.Run() }()
			defer func() {
				a.app.Stop()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("the application did not stop")
				}
			}()

			sync := func(f func()) {
				ch := make(chan struct{})
				a.app.QueueUpdateDraw(func() { f(); close(ch) })
				select {
				case <-ch:
				case <-time.After(5 * time.Second):
					t.Fatal("the event loop is wedged")
				}
			}

			// no cluster data: the views mount their text view and stream
			// nothing, which is all this test needs
			var view *tview.TextView
			sync(func() {
				a.nav(test.view)
				view = a.textView
			})
			if view == nil {
				t.Fatal("the view mounted no text view")
			}

			before := screen.shows.Load()

			// stand in for the streaming goroutine
			written := make(chan struct{})
			go func() {
				defer close(written)
				for i := 0; i < 20; i++ {
					fmt.Fprintf(view, "line %d\n", i)
				}
			}()
			<-written

			deadline := time.Now().Add(5 * time.Second)
			for screen.shows.Load() == before && time.Now().Before(deadline) {
				time.Sleep(20 * time.Millisecond)
			}
			if got := screen.shows.Load(); got == before {
				t.Errorf("writing into the %s view refreshed no screen (%d shows)", test.name, got)
			} else {
				t.Logf("%s: %d screen refreshes for 20 written lines", test.name, got-before)
			}
		})
	}
}
