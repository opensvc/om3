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
