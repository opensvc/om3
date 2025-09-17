package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"text/template"
	"time"

	"github.com/gdamore/tcell/v2"
)

var (
	eventTemplate *template.Template
)

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func formatJSON(data json.RawMessage) string {
	var parsed map[string]interface{}
	err := json.Unmarshal(data, &parsed)
	if err != nil {
		return string(data)
	}
	result, err := json.Marshal(parsed)
	if err != nil {
		return string(data)
	}
	return string(result)
}

func (t *App) getEventsViewTitle() string {
	state := ""
	if t.stopEvents {
		state = "(paused)"
	}
	return fmt.Sprintf("events %s", state)
}

func (t *App) initEventsView() {
	t.textView.SetTitle(t.getEventsViewTitle())
	t.textView.Clear()
	t.textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == ' ' {
			t.stopEvents = !t.stopEvents
			t.textView.SetTitle(t.getEventsViewTitle())
		}
		return event
	})

	eventTemplate = template.New("ev").Funcs(template.FuncMap{
		"formatTime": formatTime,
		"formatJSON": formatJSON,
	})
	eventTemplate = template.Must(eventTemplate.Parse(`{{ formatTime .At }} {{ .Kind }} [{{ .ID }}] {{ formatJSON .Data }}`))

}

func (t *App) updateEventsView() {

	if t.textView == nil {
		return
	}

	if t.eventsCancel != nil {
		t.eventsCancel()
	}

	t.eventsCtx, t.eventsCancel = context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case event := <-t.events:
				if t.stopEvents {
					continue
				}

				if t.textView == nil {
					return
				}
				err := eventTemplate.Execute(t.textView, event)

				if err != nil {
					t.errorf("%s", err)
					return
				}

				fmt.Fprintln(t.textView)
				t.textView.ScrollToEnd()
			case <-t.eventsCtx.Done():
				return
			}
		}
	}()

}
