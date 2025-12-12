package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectDisable struct {
		OptsGlobal
		commoncmd.OptsLock
		commoncmd.OptsResourceSelector
		Local bool
	}
)

func (t *CmdObjectDisable) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	if t.Local {
		return t.doObjectAction(mergedSelector)
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	sel := objectselector.New(mergedSelector, objectselector.WithClient(c))
	paths, err := sel.MustExpand()
	if err != nil {
		return err
	}
	for _, p := range paths {
		params := api.PostSvcDisableParams{}
		params.Rid = &t.RID
		params.Subset = &t.Subset
		params.Tag = &t.Tag
		response, err := c.PostSvcDisableWithResponse(context.Background(), p.Namespace, p.Name, &params)
		if err != nil {
			return err
		}
		switch response.StatusCode() {
		case 204:
		case 400:
			return fmt.Errorf("%s: %s", p, *response.JSON400)
		case 401:
			return fmt.Errorf("%s: %s", p, *response.JSON401)
		case 403:
			return fmt.Errorf("%s: %s", p, *response.JSON403)
		case 500:
			return fmt.Errorf("%s: %s", p, *response.JSON500)
		default:
			return fmt.Errorf("%s: unexpected response: %s", p, response.Status())
		}
	}
	return nil
}

func (t *CmdObjectDisable) doObjectAction(mergedSelector string) error {
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			o, err := object.NewSvc(p)
			if err != nil {
				return nil, err
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			ctx = actioncontext.WithSubset(ctx, t.Subset)
			ctx = actioncontext.WithTag(ctx, t.Tag)
			ctx = actioncontext.WithRID(ctx, t.RID)
			return nil, o.Disable(ctx)
		}),
	).Do()
}
