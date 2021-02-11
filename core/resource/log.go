package resource

import (
	"fmt"
)

type (
	// LogType holds the information, warning and alerts of a Resource
	LogType struct {
		Log []LogEntry
	}

	// LogEntryLevel can be "error", "warn", "info"
	LogEntryLevel string

	// LogEntry is an element of LogType.Log
	LogEntry struct {
		Level   LogEntryLevel `json:"level"`
		Message string        `json:"message"`
	}
)

func push(l *LogType, lvl LogEntryLevel, s string, args ...interface{}) {
	message := fmt.Sprintf(s, args...)
	entry := LogEntry{Level: lvl, Message: message}
	l.Log = append(l.Log, entry)
}

// Error append an error message to the log
func (l *LogType) Error(s string, args ...interface{}) {
	push(l, "error", s, args...)
}

// Warn append a warning message to the log
func (l *LogType) Warn(s string, args ...interface{}) {
	push(l, "warn", s, args...)
}

// Info append an info message to the log
func (l *LogType) Info(s string, args ...interface{}) {
	push(l, "info", s, args...)
}

// Dump the log entries
func (l LogType) Dump() []LogEntry {
	return l.Log
}
