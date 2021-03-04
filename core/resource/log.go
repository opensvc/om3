package resource

import (
	"fmt"
)

type (
	// Log holds the information, warning and alerts of a Resource
	Log struct {
		entries []*LogEntry
	}

	// Level can be "error", "warn", "info"
	Level string

	// LogEntry is an element of LogType.Log
	LogEntry struct {
		Level   Level  `json:"level"`
		Message string `json:"message"`
	}
)

func push(l *Log, lvl Level, s string, args ...interface{}) {
	message := fmt.Sprintf(s, args...)
	entry := &LogEntry{Level: lvl, Message: message}
	l.entries = append(l.entries, entry)
}

// Error append an error message to the log
func (l *Log) Error(s string, args ...interface{}) {
	push(l, "error", s, args...)
}

// Warn append a warning message to the log
func (l *Log) Warn(s string, args ...interface{}) {
	push(l, "warn", s, args...)
}

// Info append an info message to the log
func (l *Log) Info(s string, args ...interface{}) {
	push(l, "info", s, args...)
}

// Entries the log entries
func (l *Log) Entries() []*LogEntry {
	return l.entries
}
