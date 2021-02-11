package log

import (
	"fmt"
)

type (
	// Type holds the information, warning and alerts of a Resource
	Type struct {
		Entries []Entry
	}

	// Level can be "error", "warn", "info"
	Level string

	// Entry is an element of LogType.Log
	Entry struct {
		Level   Level  `json:"level"`
		Message string `json:"message"`
	}
)

func push(l *Type, lvl Level, s string, args ...interface{}) {
	message := fmt.Sprintf(s, args...)
	entry := Entry{Level: lvl, Message: message}
	l.Entries = append(l.Entries, entry)
}

// Error append an error message to the log
func (l *Type) Error(s string, args ...interface{}) {
	push(l, "error", s, args...)
}

// Warn append a warning message to the log
func (l *Type) Warn(s string, args ...interface{}) {
	push(l, "warn", s, args...)
}

// Info append an info message to the log
func (l *Type) Info(s string, args ...interface{}) {
	push(l, "info", s, args...)
}

// Dump the log entries
func (l Type) Dump() []Entry {
	return l.Entries
}
