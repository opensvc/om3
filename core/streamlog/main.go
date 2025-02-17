package streamlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/fatih/color"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/command"
)

type (
	Stream struct {
		cmd  *command.T
		q    chan Event
		errs chan error
	}
	StreamConfig struct {
		Follow  bool
		Lines   int
		Matches []string
	}
	Event struct {
		B []byte
		M map[string]any
	}
	Events []Event
)

func (event *Event) Map() map[string]any {
	return event.M
}

func (event *Event) IsZero() bool {
	return event.M == nil
}

func (event *Event) RenderConsole() {
	w := zerolog.NewConsoleWriter()
	w.TimeFormat = "2006-01-02T15:04:05.000Z07:00"
	w.NoColor = color.NoColor
	w.FormatFieldName = func(i any) string { return "" }
	w.FormatFieldValue = func(i any) string { return "" }
	w.FormatMessage = func(i any) string {
		return rawconfig.Colorize.Bold(i)
	}
	switch s := event.M["JSON"].(type) {
	case string:
		_, _ = w.Write([]byte(s))
	}
}

func (event *Event) RenderData() {
	fmt.Printf("%s\n", string(event.B))
}

func (event *Event) Render(format string) {
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
		_, _ = w.Write(event.B)
	}
}

func (events Events) RenderData() {
	for _, event := range events {
		fmt.Printf("%s\n", string(event.B))
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

func (events Events) Sort() {
	sort.Slice(events, func(i, j int) bool {
		var ts1, ts2 any
		var ok bool
		if ts1, ok = events[i].M["t"]; !ok {
			return false
		}
		if ts2, ok = events[j].M["t"]; !ok {
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
}

func (events Events) MatchString(key, pattern string) bool {
	for _, event := range events {
		if val, ok := event.M[key]; !ok {
			continue
		} else {
			switch s := val.(type) {
			case string:
				if v, err := regexp.MatchString(pattern, s); (err == nil) && v {
					return true
				}
			}
		}
	}
	return false
}
func NewEvent(b []byte) (Event, error) {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return Event{}, err
	} else {
		return Event{B: b, M: m}, nil
	}
}

func NewStream() *Stream {
	return &Stream{
		q:    make(chan Event),
		errs: make(chan error),
	}
}

func (stream *Stream) Errors() chan error {
	return stream.errs
}

func (stream *Stream) Events() chan Event {
	return stream.q
}

func (stream *Stream) Stop() error {
	if c := stream.cmd.Cmd(); c != nil {
		_ = c.Process.Kill()
	}
	return nil
}

func (stream *Stream) Start(streamConfig StreamConfig) error {
	comm, err := os.Executable()
	if err != nil {
		return err
	}
	var args []string
	comm = filepath.Base(comm)
	args = append(args, "-o", "json", "_COMM="+comm)
	args = append(args, streamConfig.Matches...)
	args = append(args, "-n", fmt.Sprint(streamConfig.Lines))
	if streamConfig.Follow {
		args = append(args, "-f")
	}
	stream.cmd = command.New(
		command.WithName("journalctl"),
		command.WithArgs(args),
		command.WithOnStdoutLine(func(line string) {
			if event, err := NewEvent([]byte(line)); err != nil {
				stream.errs <- err
			} else {
				stream.q <- event
			}
		}),
	)
	if err := stream.cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = stream.cmd.Wait()
		stream.errs <- nil // signal client we are done sending
	}()
	return nil
}
