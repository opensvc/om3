package streamlog

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/logging"
)

type (
	Stream struct {
		mu      sync.Mutex
		cmd     *command.T
		stopped bool
		q       chan Event
		errs    chan error
	}
	StreamConfig struct {
		Follow  bool
		Lines   int
		Matches []string
		Grep    *string
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
	w.FormatLevel = logging.FormatLevel
	w.FormatPrepare = logging.DropFields
	w.FormatMessage = func(i any) string {
		node := ""
		if nodeVal, ok := event.M["NODE"].(string); ok {
			node = rawconfig.Colorize.Bold(nodeVal + ": ")
		}
		return node + rawconfig.Colorize.Bold(i)
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
	w.FormatLevel = logging.FormatLevel
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
		if len(b) > 0 && b[len(b)-1] == '\n' {
			return Event{B: b[:len(b)-1], M: m}, nil
		} else {
			return Event{B: b, M: m}, nil
		}
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
	stream.mu.Lock()
	defer stream.mu.Unlock()
	stream.stopped = true
	if stream.cmd != nil {
		if c := stream.cmd.Cmd(); c != nil && c.Process != nil {
			_ = c.Process.Kill()
		}
	}
	return nil
}

// args returns the journalctl arguments selecting the entries of the
// config, the options of the read added.
func (streamConfig StreamConfig) args(options ...string) ([]string, error) {
	comm, err := os.Executable()
	if err != nil {
		return nil, err
	}
	args := []string{"-o", "json", "_COMM=" + filepath.Base(comm)}
	args = append(args, streamConfig.Matches...)
	// An empty pattern is no pattern. Passing --grep with one asks journalctl
	// to filter on nothing, and asks it for an option the journalctl of an
	// older distribution does not have, which fails the whole read.
	if streamConfig.Grep != nil && *streamConfig.Grep != "" {
		args = append(args, "--grep", *streamConfig.Grep)
	}
	return append(args, options...), nil
}

// newCmd returns a journalctl command sending the entries it reads to the
// stream, and its stderr as errors. onEvent is called with each entry sent.
func (stream *Stream) newCmd(args []string, onEvent func(Event)) *command.T {
	return command.New(
		command.WithName("journalctl"),
		command.WithArgs(args),
		command.WithOnStdoutLine(func(line string) {
			if event, err := NewEvent([]byte(line)); err != nil {
				stream.errs <- err
			} else {
				if onEvent != nil {
					onEvent(event)
				}
				stream.q <- event
			}
		}),
		// journalctl says why it read nothing on its stderr, and a read that
		// fails is otherwise indistinguishable from an object with no log.
		command.WithOnStderrLine(func(line string) {
			stream.errs <- fmt.Errorf("journalctl: %s", line)
		}),
	)
}

// setCmd makes cmd the command Stop kills, and says false when the stream
// is stopped already, for the command not to be started.
func (stream *Stream) setCmd(cmd *command.T) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.stopped {
		return false
	}
	stream.cmd = cmd
	return true
}

func (stream *Stream) Start(streamConfig StreamConfig) error {
	if streamConfig.Follow {
		if _, err := streamConfig.args(); err != nil {
			return err
		}
		go stream.follow(streamConfig)
		return nil
	}
	args, err := streamConfig.args("-n", fmt.Sprint(streamConfig.Lines))
	if err != nil {
		return err
	}
	cmd := stream.newCmd(args, nil)
	if !stream.setCmd(cmd) {
		return nil
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait()
		stream.errs <- nil // signal client we are done sending
	}()
	return nil
}

// follow sends the last entries of the config, then the entries logged after
// them, as they are logged.
//
// It is two reads rather than one "journalctl -n <lines> -f": with a field to
// match, as the session or the object of a log request, that one starts
// following from the tail it showed and misses entries logged in a burst
// afterwards (seen with systemd 255, where it sent 1 to 9 of 10). A follow
// starting after a cursor misses none. The cursor is the one of the last
// entry shown, or, when none was, the one of the last entry of the journal,
// taken before the entries were read: an entry logged in between is in the
// first read or after the cursor, and never in both.
func (stream *Stream) follow(streamConfig StreamConfig) {
	defer func() { stream.errs <- nil }() // signal client we are done sending
	after := lastCursor()
	if streamConfig.Lines > 0 {
		args, _ := streamConfig.args("-n", fmt.Sprint(streamConfig.Lines))
		cmd := stream.newCmd(args, func(event Event) {
			if c, ok := event.M["__CURSOR"].(string); ok && c != "" {
				after = c
			}
		})
		if !stream.setCmd(cmd) {
			return
		}
		if err := stream.run(cmd); err != nil {
			return
		}
	}
	options := []string{"-f"}
	if after != "" {
		options = append(options, "--after-cursor", after)
	} else {
		// An empty journal: what is logged from now on.
		options = append(options, "--since", "now")
	}
	args, _ := streamConfig.args(options...)
	cmd := stream.newCmd(args, nil)
	if !stream.setCmd(cmd) {
		return
	}
	_ = stream.run(cmd)
}

// run runs cmd to its end, and sends the error starting it to the stream.
func (stream *Stream) run(cmd *command.T) error {
	if err := cmd.Start(); err != nil {
		stream.errs <- err
		return err
	}
	_ = cmd.Wait()
	return nil
}

// lastCursor returns the cursor of the last entry of the journal, empty when
// the journal holds none.
func lastCursor() string {
	b, err := exec.Command("journalctl", "-n", "1", "-o", "json").Output()
	if err != nil {
		return ""
	}
	var m map[string]any
	for _, line := range strings.Split(string(b), "\n") {
		if err := json.Unmarshal([]byte(line), &m); err == nil {
			if c, ok := m["__CURSOR"].(string); ok {
				return c
			}
		}
	}
	return ""
}
