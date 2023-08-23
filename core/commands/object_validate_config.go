package commands

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/path"
)

type (
	CmdObjectValidateConfig struct {
		OptsGlobal
		OptsLock
	}
)

func (t *CmdObjectValidateConfig) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("validate config"),
		objectaction.WithLocalRun(func(ctx context.Context, p path.T) (interface{}, error) {
			o, err := object.New(p)
			if err != nil {
				return nil, err
			}
			c, ok := o.(object.Configurer)
			if !ok {
				return nil, fmt.Errorf("%s is not a configurer", o)
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			return c.ValidateConfig(ctx)
		}),
	).Do()
}
