package slog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/fatih/color"
	"github.com/hpcloud/tail"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/xerrors"
)

type (
	Stream struct {
		tails    []*tail.Tail
		controls []chan bool
		q        chan Event
	}
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

func (event Event) RenderConsole() {
	w := zerolog.NewConsoleWriter()
	w.TimeFormat = "2006-01-02T15:04:05.000Z07:00"
	w.NoColor = color.NoColor
	_, _ = w.Write(event.b)
}

func (event Event) RenderData() {
	fmt.Printf("%s\n", string(event.b))
}

func (event Event) Render(format string) {
	switch format {
	case "json":
		event.RenderData()
	default:
		event.RenderConsole()
	}
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

func NewEvent(b []byte) (Event, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return Event{}, err
	} else {
		return Event{b: b, m: m}, nil
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
		b := []byte(scanner.Text())
		if event, err := NewEvent(b); err != nil {
			continue
		} else if event.IsMatching(filters) {
			events = append(events, event)
		}
	}
	return events, nil
}

func NewStream() *Stream {
	return &Stream{
		q:        make(chan Event),
		tails:    make([]*tail.Tail, 0),
		controls: make([]chan bool, 0),
	}
}

func (stream Stream) Events() chan Event {
	return stream.q
}

func (stream *Stream) Stop() error {
	var errs error
	for _, control := range stream.controls {
		control <- true
	}
	for _, t := range stream.tails {
		if err := t.Stop(); err != nil {
			xerrors.Append(errs, err)
		}
	}
	return errs
}

func (stream *Stream) Follow(fpath string) error {
	t, err := tail.TailFile(fpath, tail.Config{Follow: true, ReOpen: true})
	if err != nil {
		return err
	}
	control := make(chan bool)
	stream.controls = append(stream.controls, control)
	stream.tails = append(stream.tails, t)
	go func() {
		for {
			select {
			case line := <-t.Lines:
				if event, err := NewEvent([]byte(line.Text)); err == nil {
					stream.q <- event
				}
			case _ = <-control:
				return
			}
		}
	}()
	return nil
}

func GetEventStreamFromNode(filters map[string]interface{}) (*Stream, error) {
	files := []string{filepath.Join(rawconfig.Paths.Log, "node.log")}
	return GetEventStreamFromFiles(files, filters)
}

func GetEventStreamFromObjects(paths []path.T, filters map[string]interface{}) (*Stream, error) {
	files := make([]string, len(paths))
	for i := 0; i < len(paths); i += 1 {
		files[i] = paths[i].LogFile()
	}
	return GetEventStreamFromFiles(files, filters)
}

func GetEventStreamFromFiles(files []string, filters map[string]interface{}) (*Stream, error) {
	stream := NewStream()
	var errs error
	for _, p := range files {
		if err := stream.Follow(p); err != nil {
			xerrors.Append(errs, err)
		}
	}
	return stream, errs
}

func GetEventsFromNode(filters map[string]interface{}) (Events, error) {
	file := filepath.Join(rawconfig.Paths.Log, "node.log")
	return GetEventsFromFile(file, filters)
}

func GetEventsFromObjects(paths []path.T, filters map[string]interface{}) (Events, error) {
	events := make(Events, 0)
	var errs error
	for _, p := range paths {
		fpath := p.LogFile()
		more, err := GetEventsFromFile(fpath, filters)
		if err != nil {
			xerrors.Append(errs, err)
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
		sts1, ok1 := ts1.(string)
		sts2, ok2 := ts2.(string)
		if ok1 && ok2 {
			return sts1 < sts2
		}
		fts1, ok1 := ts1.(float64)
		fts2, ok2 := ts2.(float64)
		if ok1 && ok2 {
			return fts1 < fts2
		}
		return false
	})
	return events, errs
}
