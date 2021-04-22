package resource

import (
	"fmt"
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
		Level   Level  `json:"level"`
		Message string `json:"message"`
	}
)

func push(l *StatusLog, lvl Level, s string, args ...interface{}) {
	message := fmt.Sprintf(s, args...)
	entry := &StatusLogEntry{Level: lvl, Message: message}
	l.entries = append(l.entries, entry)
}

// Error append an error message to the log
func (l *StatusLog) Error(s string, args ...interface{}) {
	push(l, "error", s, args...)
}

// Warn append a warning message to the log
func (l *StatusLog) Warn(s string, args ...interface{}) {
	push(l, "warn", s, args...)
}

// Info append an info message to the log
func (l *StatusLog) Info(s string, args ...interface{}) {
	push(l, "info", s, args...)
}

// Entries the log entries
func (l *StatusLog) Entries() []*StatusLogEntry {
	return l.entries
}
