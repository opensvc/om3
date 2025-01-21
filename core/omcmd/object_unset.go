package omcmd

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/key"
)

type (
	CmdObjectUnset struct {
		OptsGlobal
		OptsLock
		Keywords []string
		Sections []string
	}
)

func (t *CmdObjectUnset) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
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
		params := api.PostObjectConfigUpdateParams{}
		params.Unset = &t.Keywords
		params.Delete = &t.Sections
		response, err := c.PostObjectConfigUpdateWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return err
		}
		switch response.StatusCode() {
		case 204:
			fmt.Printf("%s: committed\n", p)
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

func (t *CmdObjectUnset) doObjectAction(mergedSelector string) error {

	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			// TODO: one commit on Unset, one commit on DeleteSection. Change to single commit ?
			o, err := object.NewConfigurer(p)
			if err != nil {
				return nil, err
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			kws := key.ParseStrings(t.Keywords)
			var changed bool
			if len(kws) > 0 {
				log.Debug().Msgf("unsetting %s keywords: %s", p, kws)
				if err = o.Unset(ctx, kws...); err != nil {
					return nil, err
				}
				changed = true
			}
			sections := make([]string, 0)
			for _, r := range t.Sections {
				if r != "DEFAULT" {
					sections = append(sections, r)
				}
			}
			if len(sections) > 0 {
				log.Debug().Msgf("deleting %s sections: %s", p, sections)
				if err = o.DeleteSection(ctx, sections...); err != nil {
					return nil, err
				}
				changed = true
			}
			if changed {
				fmt.Printf("%s: committed\n", p)
			}
			return nil, nil
		}),
	).Do()
}
