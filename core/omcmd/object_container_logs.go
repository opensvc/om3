package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/core/resourceid"
)

type (
	CmdObjectContainerLogs struct {
		OptsGlobal
		RID           string
		Follow        bool
		Lines         int
	}
)

func (t *CmdObjectContainerLogs) Run(kind string) error {
	if t.RID == "" {
		return fmt.Errorf("rid is required for container logs")
	}

	rid, err := resourceid.Parse(t.RID)
	if err != nil {
		return fmt.Errorf("invalid rid: %w", err)
	}

	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			return nil, o.ContainerLogs(ctx, rid.String(), t.Follow, t.Lines)
		}),
	).Do()
}
