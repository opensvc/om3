package oxcmd

import (
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectKeyChange struct {
		OptsGlobal
		commoncmd.OptsLock
		Name  string
		From  *string
		Value *string
		Wait  bool
		Time  time.Duration
	}
)

func (t *CmdObjectKeyChange) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	if t.Value == nil && t.From == nil {
		return fmt.Errorf("a value or value source mut be specified for a change action")
	}
	data, err := commoncmd.MakeDataPatch(t.Name, t.Value, t.From, api.Change)
	if err != nil {
		return err
	}
	return commoncmd.RunObjectDataPatch(mergedSelector, data, t.Wait, t.Time)
}
