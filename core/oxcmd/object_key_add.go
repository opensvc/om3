package oxcmd

import (
	"time"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectKeyAdd struct {
		OptsGlobal
		commoncmd.OptsLock
		Name  string
		From  *string
		Value *string
		Wait  bool
		Time  time.Duration
	}
)

func (t *CmdObjectKeyAdd) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	if t.Value == nil && t.From == nil {
		s := ""
		t.Value = &s
	}
	data, err := commoncmd.MakeDataPatch(t.Name, t.Value, t.From, api.Add)
	if err != nil {
		return err
	}
	return commoncmd.RunObjectDataPatch(mergedSelector, data, t.Wait, t.Time)
}
