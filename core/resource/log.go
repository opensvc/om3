package resource

import (
	"fmt"
)

type (
	LogType struct {
		Log		[]LogEntry
	}

	LogEntryLevel		string

	LogEntry struct {
		Level		LogEntryLevel	`json:"level"`
		Message		string		`json:"message"`
	}
)

func (l *LogType) Error(s string, args ...interface{}) {
	message := fmt.Sprintf(s, args...)
	entry := LogEntry{Level: "error", Message: message}
	l.Log = append(l.Log, entry)
}

func (l LogType) Dump() []LogEntry {
	return l.Log
}
