package omcmd

import (
	"context"
	"encoding/json"
	"os"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectInstancePrefetch struct {
		OptsGlobal
		RID string
	}
)

// Run prefetches the images of the local instances selected, and prints
// what each prefetch did, a json list per object.
//
// A prefetch that failed is an outcome, printed and recorded for the daemon and
// the resource status: the command fails only when a prefetch could not be
// tried.
func (t *CmdObjectInstancePrefetch) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			reports, err := o.Prefetch(ctx, t.RID)
			if encodeErr := json.NewEncoder(os.Stdout).Encode(reports); encodeErr != nil && err == nil {
				err = encodeErr
			}
			return nil, err
		}),
	).Do()
}
