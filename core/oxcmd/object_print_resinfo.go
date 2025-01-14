package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectPrintResourceInfo struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdObjectPrintResourceInfo) Run(selector, kind string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	paths, err := objectselector.New(mergedSelector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	l := make(api.ResourceInfoItems, 0)
	q := make(chan api.ResourceInfoItems)
	errC := make(chan error)
	doneC := make(chan string)
	todoP := len(paths)
	for _, path := range paths {
		go func(p naming.Path) {
			defer func() { doneC <- p.String() }()
			response, err := c.GetObjectResourceInfoWithResponse(ctx, p.Namespace, p.Kind, p.Name)
			if err != nil {
				errC <- err
				return
			}
			switch {
			case response.JSON200 != nil:
				q <- response.JSON200.Items
			case response.JSON401 != nil:
				errC <- fmt.Errorf("%s: %s", p, *response.JSON401)
			case response.JSON403 != nil:
				errC <- fmt.Errorf("%s: %s", p, *response.JSON403)
			case response.JSON500 != nil:
				errC <- fmt.Errorf("%s: %s", p, *response.JSON500)
			default:
				errC <- fmt.Errorf("%s: unexpected response: %s", p, response.Status())
			}
		}(path)
	}

	var (
		errs  error
		doneP int
	)

	for {
		select {
		case err := <-errC:
			errs = errors.Join(errs, err)
		case items := <-q:
			l = append(l, items...)
		case <-doneC:

			if !(doneP == todoP) {
				doneP++
			}

			if doneP == todoP {
				goto out
			}

		case <-ctx.Done():
			errs = errors.Join(errs, ctx.Err())
			goto out
		}
	}

out:

	output.Renderer{
		DefaultOutput: "tab=OBJECT:object,NODE:node,RID:rid,KEY:key,VALUE:value",
		Output:        t.Output,
		Color:         t.Color,
		Data:          api.ResourceInfoList{Items: l, Kind: "ResourceInfoList"},
		Colorize:      rawconfig.Colorize,
	}.Print()
	return errs
}
