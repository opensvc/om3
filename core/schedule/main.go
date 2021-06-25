package schedule

import (
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	Table []Entry

	Entry struct {
		Path       path.T      `json:"path"`
		Node       string      `json:"node"`
		Action     string      `json:"action"`
		Key        string      `json:"config_parameter"`
		Last       timestamp.T `json:"last_run"`
		Next       timestamp.T `json:"next_run"`
		Definition string      `json:"schedule_definition"`
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
