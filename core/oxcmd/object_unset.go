package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
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
	c, err := client.New()
	if err != nil {
		return err
	}
	sel := objectselector.New(mergedSelector, objectselector.WithClient(c))
	paths, err := sel.MustExpand()
	if err != nil {
		return err
	}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	errC := make(chan error)
	doneC := make(chan string)
	todo := len(paths)

	for _, path := range paths {
		go func(p naming.Path) {
			defer func() { doneC <- p.String() }()
			params := api.PostObjectConfigUpdateParams{}
			params.Unset = &t.Keywords
			params.Delete = &t.Sections
			response, err := c.PostObjectConfigUpdateWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
			if err != nil {
				errC <- err
				return
			}
			switch response.StatusCode() {
			case 204:
				fmt.Printf("%s: commited\n", p)
			case 400:
				errC <- fmt.Errorf("%s: %s", p, *response.JSON400)
			case 401:
				errC <- fmt.Errorf("%s: %s", p, *response.JSON401)
			case 403:
				errC <- fmt.Errorf("%s: %s", p, *response.JSON403)
			case 500:
				errC <- fmt.Errorf("%s: %s", p, *response.JSON500)
			default:
				errC <- fmt.Errorf("%s: unexpected response: %s", p, response.Status())
			}
		}(path)
	}

	var (
		errs error
		done int
	)

	for {
		select {
		case err := <-errC:
			errs = errors.Join(errs, err)
		case <-doneC:
			done++
			if done == todo {
				return errs
			}
		case <-ctx.Done():
			errs = errors.Join(errs, ctx.Err())
			return errs
		}
	}
}
