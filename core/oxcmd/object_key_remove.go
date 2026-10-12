package oxcmd

import (
	"time"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectKeyRemove struct {
		OptsGlobal
		Names []string
		Wait  bool
		Time  time.Duration
	}
)

func (t *CmdObjectKeyRemove) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	data := make(api.PatchDataKeys, len(t.Names))
	for i, name := range t.Names {
		data[i] = api.PatchDataKey{
			Action: api.Remove,
			Name:   name,
		}
	}
	return commoncmd.RunObjectDataPatch(mergedSelector, data, t.Wait, t.Time)
}
