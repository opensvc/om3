package slog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
)

type (
	Event struct {
		b []byte
		m map[string]interface{}
	}
	Events []Event
)

func (event Event) Map() map[string]interface{} {
	return event.m
}

func (event Event) IsMatching(filters map[string]interface{}) bool {
	for k, v := range filters {
		if current, ok := event.m[k]; !ok || (current != v) {
			return false
		}
	}
	return true
}

func (events Events) RenderConsole() {
	w := zerolog.NewConsoleWriter()
	w.TimeFormat = "2006-01-02T15:04:05.000Z07:00"
	w.NoColor = color.NoColor
	for _, event := range events {
		_, _ = w.Write(event.b)
	}
}

func (events Events) RenderData() {
	for _, event := range events {
		fmt.Printf("%s\n", string(event.b))
	}
}

func (events Events) Render(format string) {
	switch format {
	case "json":
		events.RenderData()
	default:
		events.RenderConsole()
	}
}
func GetEventsFromFile(fpath string, filters map[string]interface{}) (Events, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	events := make(Events, 0)
	for scanner.Scan() {
		event := Event{}
		event.b = scanner.Bytes()
		err := json.Unmarshal(event.b, &event.m)
		if err != nil {
			continue
		}
		if event.IsMatching(filters) {
			events = append(events, event)
		}
	}
	return events, nil
}

func GetEventsFromObjects(paths []path.T, filters map[string]interface{}) (Events, error) {
	events := make(Events, 0)
	for _, p := range paths {
		fpath := object.LogFile(p)
		more, err := GetEventsFromFile(fpath, filters)
		if err != nil {
			return events, err
		}
		events = append(events, more...)
	}
	sort.Slice(events, func(i, j int) bool {
		var ts1, ts2 interface{}
		var ok bool
		if ts1, ok = events[i].m["t"]; !ok {
			return false
		}
		if ts2, ok = events[j].m["t"]; !ok {
			return true
		}
		return ts1.(float64) < ts2.(float64)
	})
	return events, nil
}
