package commands

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/key"
)

type (
	CmdObjectUnset struct {
		OptsGlobal
		OptsLock
		Keywords []string
	}
)

func (t *CmdObjectUnset) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("unset"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"kw": t.Keywords,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewConfigurer(p)
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			kws := key.ParseL(t.Keywords)
			return nil, o.Unset(ctx, kws...)
		}),
	).Do()
}
