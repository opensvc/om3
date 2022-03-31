package compliance

import (
	"encoding/json"
	"sync"
)

type (
	LogLevel int
	LogEntry struct {
		Level LogLevel
		Msg   string
	}
	LogEntries struct {
		entries []LogEntry
		mu      sync.Mutex
	}
)

const (
	LogLevelOut LogLevel = 0
	LogLevelErr LogLevel = 1
)

func NewLogEntries() *LogEntries {
	return &LogEntries{
		entries: make([]LogEntry, 0),
	}
}

func (t *LogEntries) Entries() []LogEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	l := make([]LogEntry, len(t.entries))
	for i, e := range t.entries {
		l[i] = e
	}
	return l
}

func (t *LogEntries) Out(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, LogEntry{
		Level: LogLevelOut,
		Msg:   s,
	})
}

func (t *LogEntries) Err(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, LogEntry{
		Level: LogLevelErr,
		Msg:   s,
	})
}

// MarshalJSON marshals the data as a quoted json string
func (t LogEntries) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.entries)
}

// UnmarshalJSON unmashals a quoted json string to value
func (t *LogEntries) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, t.entries)
}
