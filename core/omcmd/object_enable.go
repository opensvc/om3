package omcmd

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectEnable struct {
		OptsGlobal
		OptsLock
		OptsResourceSelector
	}
)

func (t *CmdObjectEnable) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	if t.Local {
		return t.doObjectAction(mergedSelector)
	}
	panic("TODO")
	/*
		c, err := client.New()
		if err != nil {
			return err
		}
		sel := objectselector.New(mergedSelector, objectselector.WithClient(c))
		paths, err := sel.Expand()
		if err != nil {
			return err
		}
		for _, p := range paths {
			params := api.PostObjectConfigUpdateParams{}
			params.Unset = &t.Keywords
			params.Delete = &t.Sections
			response, err := c.PostObjectConfigUpdateWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
			if err != nil {
				return err
			}
			switch response.StatusCode() {
			case 204:
				fmt.Printf("%s: commited\n", p)
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
	*/
	return nil
}

func (t *CmdObjectEnable) doObjectAction(mergedSelector string) error {
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
			return nil, o.Enable(ctx)
		}),
	).Do()
}
