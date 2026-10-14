package logging

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

func TestDropFields(t *testing.T) {
	var buf bytes.Buffer
	w := zerolog.ConsoleWriter{
		Out:           &buf,
		NoColor:       true,
		PartsOrder:    []string{zerolog.LevelFieldName, zerolog.MessageFieldName},
		FormatPrepare: DropFields,
	}
	line := `{"level":"info","message":"hello","node":"n1","path":"ns/svc/foo","rid":"disk#2"}`
	if _, err := w.Write([]byte(line)); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "INF hello\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The plain format shows a record as its message alone, as a line a command
// prints, with its level before it from the warnings up.
func TestPlainConsoleFormat(t *testing.T) {
	var buf bytes.Buffer
	w := zerolog.ConsoleWriter{
		Out:           &buf,
		NoColor:       true,
		PartsOrder:    []string{zerolog.LevelFieldName, zerolog.MessageFieldName},
		FormatLevel:   FormatPlainLevel,
		FormatPrepare: DropFields,
	}
	for _, tc := range []struct {
		line, want string
	}{
		{`{"level":"info","time":"2026-10-14T18:41:13Z","message":"system/sec/ca-next: set key certificate","path":"system/sec/ca-next"}`, "system/sec/ca-next: set key certificate\n"},
		{`{"level":"warn","message":"careful"}`, "WRN careful\n"},
	} {
		buf.Reset()
		if _, err := w.Write([]byte(tc.line)); err != nil {
			t.Fatal(err)
		}
		if got := buf.String(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
