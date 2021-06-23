package schedule

import (
	"time"

	"opensvc.com/opensvc/core/path"
)

type (
	Table []Entry

	Entry struct {
		Path       path.T    `json:"path"`
		Node       string    `json:"node"`
		Action     string    `json:"action"`
		Key        string    `json:"key"`
		Last       time.Time `json:"last"`
		Next       time.Time `json:"next"`
		Definition string    `json:"definition"`
	}
)

func NewTable(entries ...Entry) Table {
	t := make([]Entry, 0)
	return Table(t).AddEntries(entries)
}

func (t Table) Add(i interface{}) Table {
	switch o := i.(type) {
	case Entry:
		return t.AddEntry(o)
	case Table:
		return t.AddTable(o)
	case []Entry:
		return t.AddEntries(o)
	default:
		return t
	}
}

func (t Table) AddTable(l Table) Table {
	return append(t, l...)
}

func (t Table) AddEntries(l []Entry) Table {
	return append(t, l...)
}

func (t Table) AddEntry(e Entry) Table {
	return append(t, e)
}
