package schedule

import (
	"time"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/key"
	usched "github.com/opensvc/om3/util/schedule"
)

type (
	Table []Entry

	Entry struct {
		Action             string    `json:"action" yaml:"action"`
		Definition         string    `json:"schedule_definition" yaml:"schedule_definition"`
		Key                string    `json:"config_parameter" yaml:"config_parameter"`
		LastRunAt          time.Time `json:"last_run_at" yaml:"last_run_at"`
		LastRunFile        string    `json:"last_run_file" yaml:"last_run_file"`
		LastSuccessFile    string    `json:"last_success_file" yaml:"last_success_file"`
		NextRunAt          time.Time `json:"next_run_at" yaml:"next_run_at"`
		Node               string    `json:"node" yaml:"node"`
		Path               path.T    `json:"path" yaml:"path"`
		RequireCollector   bool      `json:"require_collector" yaml:"require_collector"`
		RequireProvisioned bool      `json:"require_provisioned" yaml:"require_provisioned"`
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
