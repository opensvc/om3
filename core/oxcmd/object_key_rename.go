package oxcmd

import (
	"time"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectKeyRename struct {
		OptsGlobal
		Name string
		To   string
		Wait bool
		Time time.Duration
	}
)

func (t *CmdObjectKeyRename) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	data := api.PatchDataKeys{
		api.PatchDataKey{
			Name:   t.Name,
			To:     &t.To,
			Action: api.Rename,
		},
	}
	return commoncmd.RunObjectDataPatch(mergedSelector, data, t.Wait, t.Time)
}
