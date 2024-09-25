package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectPushResInfo struct {
		OptsGlobal
		OptsLock
		OptsResourceSelector
		OptTo
		NodeSelector string
	}
)

func (t *CmdObjectPushResInfo) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRID(t.RID),
		objectaction.WithTag(t.Tag),
		objectaction.WithSubset(t.Subset),
		objectaction.WithLocal(true),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteFunc(func(ctx context.Context, p naming.Path, nodename string) (any, error) {
			return nil, fmt.Errorf("TODO")
		}),
	).Do()
}
