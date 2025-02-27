package schedule

import (
	"time"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/key"
	usched "github.com/opensvc/om3/util/schedule"
)

type (
	Table []Entry

	Entry struct {
		Action             string      `json:"action"`
		Schedule           string      `json:"schedule"`
		Key                string      `json:"key"`
		LastRunAt          time.Time   `json:"last_run_at"`
		LastRunFile        string      `json:"last_run_file"`
		LastSuccessFile    string      `json:"last_success_file"`
		MaxParallel        int         `json:"max_parallel"`
		NextRunAt          time.Time   `json:"next_run_at"`
		Node               string      `json:"node"`
		Path               naming.Path `json:"path"`
		Require            string      `json:"require"`
		RequireCollector   bool        `json:"require_collector"`
		RequireProvisioned bool        `json:"require_provisioned"`
		RunDir             string      `json:"run_dir"`
	}
)

func NewTable(entries ...Entry) Table {
	t := make([]Entry, 0)
	return Table(t).AddEntries(entries...)
}

func (t Table) Merge(i interface{}) Table {
	switch o := i.(type) {
	case Table:
		return t.MergeEntries(o...)
	case Entry:
		return t.MergeEntries(o)
	case []Entry:
		return t.MergeEntries(o...)
	default:
		return t
	}
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

func (t Table) MergeEntry(e Entry) Table {
	for i, x := range t {
		if (x.Path == e.Path) && (x.Node == e.Node) && (x.Key == e.Key) {
			t[i] = e
			return t
		}
	}
	return append(t, e)
}

func (t Table) DelEntry(e Entry) Table {
	for i, x := range t {
		if (x.Path == e.Path) && (x.Node == e.Node) && (x.Key == e.Key) {
			return append(t[:i], t[i+1:]...)
		}
	}
	return t
}

func (t Table) MergeEntries(l ...Entry) Table {
	for _, e := range l {
		t = t.MergeEntry(e)
	}
	return t
}

func (t Table) AddEntries(l ...Entry) Table {
	return append(t, l...)
}

func (t Table) DeepCopy() *Table {
	r := make(Table, 0, len(t))
	for _, x := range t {
		r = append(r, Entry{
			Action:             x.Action,
			Schedule:           x.Schedule,
			Key:                x.Key,
			LastRunAt:          x.LastRunAt,
			LastRunFile:        x.LastRunFile,
			LastSuccessFile:    x.LastSuccessFile,
			MaxParallel:        x.MaxParallel,
			NextRunAt:          x.NextRunAt,
			Node:               x.Node,
			Path:               x.Path,
			Require:            x.Require,
			RequireCollector:   x.RequireCollector,
			RequireProvisioned: x.RequireProvisioned,
			RunDir:             x.RunDir,
		})
	}
	return &r
}

func (t Entry) GetNext() (time.Time, time.Duration, error) {
	sc := usched.New(t.Schedule)
	return sc.Next(usched.NextWithLast(t.LastRunAt))
}

func (t Entry) RID() string {
	k := key.Parse(t.Key)
	return k.Section
}

func (t Entry) SetLastSuccess(tm time.Time) error {
	return file.Touch(t.LastSuccessFile, tm)
}

func (t Entry) SetLastRun(tm time.Time) error {
	return file.Touch(t.LastRunFile, tm)
}
