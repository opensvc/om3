package resource

import (
	"fmt"
)

type (
	// StatusLog holds the information, warning and alerts of a Resource
	StatusLog struct {
		entries []StatusLogEntry
	}

	// Level can be "error", "warn", "info"
	Level string

	// StatusLogEntry is an element of LogType.Log
	StatusLogEntry struct {
		Level   Level  `json:"level"`
		Message string `json:"message"`
	}
)

var (
	InfoLevel  Level = "info"
	WarnLevel  Level = "warn"
	ErrorLevel Level = "error"
)

func (t StatusLogEntry) String() string {
	return fmt.Sprintf("%s: %s", t.Level, t.Message)
}

func push(l *StatusLog, lvl Level, s string, args ...any) {
	message := fmt.Sprintf(s, args...)
	entry := StatusLogEntry{Level: lvl, Message: message}
	l.entries = append(l.entries, entry)
}

func NewStatusLog(entries ...StatusLogEntry) *StatusLog {
	return &StatusLog{
		entries: entries,
	}
}

func (l *StatusLog) Len() int {
	return len(l.entries)
}

func (l *StatusLog) Merge(other StatusLogger) {
	if other == nil {
		return
	}
	l.entries = append(l.entries, other.Entries()...)
}

func (l *StatusLog) Entries() []StatusLogEntry {
	return l.entries
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
