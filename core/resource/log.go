package resource

import (
	"fmt"
	"time"
)

type (
	// StatusLog holds the information, warning and alerts of a Resource
	StatusLog struct {
		entries []StatusLogEntry

		// changeAt is the earliest time the status evaluated changes
		// with no event to tell, as a copy aging past its delay.
		changeAt time.Time

		// rpoBreachAt is the earliest time a copy the resource keeps on
		// this node breaches the recovery point objective of its
		// contract, past or not.
		rpoBreachAt time.Time
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
	if t.Level == InfoLevel {
		return t.Message
	} else {
		return fmt.Sprintf("%s: %s", t.Level, t.Message)
	}
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
	l.changeAt = time.Time{}
	l.rpoBreachAt = time.Time{}
}

// RPOBreachesAt records that a copy the resource keeps on this node, as the
// one a sync received, breaches its recovery point objective at tm: were the
// node to take over from then on, more data would be lost than its contract
// allows. The earliest time recorded is kept, past or not.
func (l *StatusLog) RPOBreachesAt(tm time.Time) {
	if l.rpoBreachAt.IsZero() || tm.Before(l.rpoBreachAt) {
		l.rpoBreachAt = tm
	}
}

// RPOBreachAt is the earliest time recorded by RPOBreachesAt, zero if none.
func (l *StatusLog) RPOBreachAt() time.Time {
	return l.rpoBreachAt
}

// ChangesAt records that the status evaluated changes at tm with no event to
// tell, for the daemon to evaluate it again then. The earliest of the times
// recorded is kept, and a time already past is ignored: the status evaluated
// has seen it.
func (l *StatusLog) ChangesAt(tm time.Time) {
	if !tm.After(time.Now()) {
		return
	}
	if l.changeAt.IsZero() || tm.Before(l.changeAt) {
		l.changeAt = tm
	}
}

// ChangeAt is the earliest time recorded by ChangesAt, zero if none.
func (l *StatusLog) ChangeAt() time.Time {
	return l.changeAt
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
