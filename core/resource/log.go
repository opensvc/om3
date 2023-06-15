package resource

import (
	"encoding/json"
	"fmt"
	"strings"
)

type (
	// StatusLog holds the information, warning and alerts of a Resource
	StatusLog struct {
		entries []*StatusLogEntry
	}

	// Level can be "error", "warn", "info"
	Level string

	// StatusLogEntry is an element of LogType.Log
	StatusLogEntry struct {
		Level   Level  `json:"level" yaml:"level"`
		Message string `json:"message" yaml:"message"`
	}
)

func (t StatusLogEntry) String() string {
	return fmt.Sprintf("%s: %s", t.Level, t.Message)
}

func push(l *StatusLog, lvl Level, s string, args ...any) {
	message := fmt.Sprintf(s, args...)
	entry := &StatusLogEntry{Level: lvl, Message: message}
	l.entries = append(l.entries, entry)
}

func (l *StatusLog) Reset() {
	l.entries = l.entries[:0]
}

// Error append an error message to the log
func (l *StatusLog) Error(s string, args ...any) {
	push(l, "error", s, args...)
}

// Warn append a warning message to the log
func (l *StatusLog) Warn(s string, args ...any) {
	push(l, "warn", s, args...)
}

// Info append an info message to the log
func (l *StatusLog) Info(s string, args ...any) {
	push(l, "info", s, args...)
}

// Entries the log entries
func (l *StatusLog) Entries() []*StatusLogEntry {
	return l.entries
}

func (t *StatusLogEntry) UnmarshalJSON(data []byte) error {
	// native format: {"level":"info","message":"foo"}
	type tempT StatusLogEntry
	var temp tempT
	if err := json.Unmarshal(data, &temp); err == nil {
		t.Level = temp.Level
		t.Message = temp.Message
		return nil
	}

	// deprecated format: "info: foo"
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	splitN := strings.SplitN(s, ":", 2)
	if len(splitN) != 2 {
		return fmt.Errorf("unmarshal StatusLogEntry")
	}
	t.Level = Level(splitN[0])
	t.Message = strings.TrimSpace(splitN[1])
	return nil
}
