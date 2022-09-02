package schedule

import (
	"time"

	"opensvc.com/opensvc/core/path"
	usched "opensvc.com/opensvc/util/schedule"
)

type (
	Table []Entry

	Entry struct {
		Path       path.T    `json:"path"`
		Node       string    `json:"node"`
		Action     string    `json:"action"`
		Key        string    `json:"config_parameter"`
		Last       time.Time `json:"last_run"`
		Next       time.Time `json:"next_run"`
		Definition string    `json:"schedule_definition"`
	}
)

func NewTable(entries ...Entry) Table {
	t := make([]Entry, 0)
	return Table(t).AddEntries(entries...)
}

func (t Table) Add(i interface{}) Table {
	switch o := i.(type) {
	case Table:
		return t.AddTable(o)
	case Entry:
		return t.AddEntries(o)
	case []Entry:
		return t.AddEntries(o...)
	default:
		return t
	}
}

func (t Table) AddTable(l Table) Table {
	return append(t, l...)
}

func (t Table) AddEntries(l ...Entry) Table {
	return append(t, l...)
}

func (t Entry) GetNext() (time.Time, time.Duration, error) {
	sc := usched.New(t.Definition)
	return sc.Next(usched.NextWithLast(t.Last))
}
