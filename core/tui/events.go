package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/gdamore/tcell/v2"
)

var (
	eventTemplate *template.Template
)

func formatJSON(data json.RawMessage) string {
	return string(data)
}

func (t *App) getEventsViewTitle() string {
	state := ""
	if t.stopEvents.Load() {
		state = "(paused)"
	}
	return fmt.Sprintf("events %s", state)
}

func (t *App) initEventsView() {
	t.textView.SetTitle(t.getEventsViewTitle())
	t.textView.Clear()
	t.textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == ' ' {
			t.stopEvents.Store(!t.stopEvents.Load())
			t.textView.SetTitle(t.getEventsViewTitle())
		}
		return event
	})

	eventTemplate = template.New("ev").Funcs(template.FuncMap{
		"formatJSON": formatJSON,
	})
	eventTemplate = template.Must(eventTemplate.Parse(`{{ .At }} {{ .Kind }} [{{ .ID }}] {{ formatJSON .Data }}`))
}

func (t *App) updateEventsView() {
	if t.textView == nil {
		return
	}

	if t.eventsCancel != nil {
		t.eventsCancel()
	}

	var ctx context.Context
	ctx, t.eventsCancel = context.WithCancel(context.Background())

	// Hand the text view and the context over to the goroutine: t.textView is
	// nil'ed by the view leave hook, on the tview loop.
	view := t.textView

	go func() {
		for {
			select {
			case event := <-t.events:
				if t.stopEvents.Load() {
					continue
				}
				if err := eventTemplate.Execute(view, event); err != nil {
					t.errorf("%s", err)
					return
				}
				fmt.Fprintln(view)
				view.ScrollToEnd()
			case <-ctx.Done():
				return
			}
		}
	}()
}
