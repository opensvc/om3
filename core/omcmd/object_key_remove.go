package omcmd

import (
	"context"
	"time"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectKeyRemove struct {
		OptsGlobal
		Names []string

		// Wait, a write through the daemon, held until the configuration
		// reaches every live node of the object, for no longer than Time.
		Wait bool
		Time time.Duration
	}
)

func (t *CmdObjectKeyRemove) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	if t.Wait {
		// Waiting for the configuration to reach the peers needs the
		// daemon, which is what propagates it: the write goes through it.
		data := make(api.PatchDataKeys, len(t.Names))
		for i, name := range t.Names {
			data[i] = api.PatchDataKey{Action: api.Remove, Name: name}
		}
		return commoncmd.RunObjectDataPatch(mergedSelector, data, t.Wait, t.Time)
	}
	return objectaction.New(
		objectaction.WithColor(t.Color),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
		objectaction.WithOutput(t.Output),
		objectaction.WithSort(t.Sort),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			store, err := object.NewDataStore(p)
			if err != nil {
				return nil, err
			}
			for _, name := range t.Names {
				if err := store.TransactionRemoveKey(name); err != nil {
					return nil, err
				}
			}
			return nil, store.Config().CommitInvalid()
		}),
	).Do()
}
