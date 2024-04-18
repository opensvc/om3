package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectPrintSchedule struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdObjectPrintSchedule) Run(selector, kind string) error {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	paths, err := objectselector.New(mergedSelector, objectselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	l := make(api.ScheduleItems, 0)
	q := make(chan api.ScheduleItems)
	errC := make(chan error)
	doneC := make(chan [2]string)
	todoP := len(paths)
	todoN := len(nodenames)
	for _, nodename := range nodenames {
		for _, path := range paths {
			go func(n string, p naming.Path) {
				defer func() { doneC <- [2]string{n, p.String()} }()
				response, err := c.GetObjectScheduleWithResponse(ctx, n, p.Namespace, p.Kind, p.Name)
				if err != nil {
					errC <- err
					return
				}
				switch {
				case response.JSON200 != nil:
					q <- response.JSON200.Items
				case response.JSON401 != nil:
					errC <- fmt.Errorf("%s: %s", n, *response.JSON401)
				case response.JSON403 != nil:
					errC <- fmt.Errorf("%s: %s", n, *response.JSON403)
				default:
					errC <- fmt.Errorf("%s: unexpected response: %s", n, response.Status())
				}
			}(nodename, path)
		}
	}

	var (
		errs  error
		doneP int
		doneN int
	)

	for {
		select {
		case err := <-errC:
			errs = errors.Join(errs, err)
		case items := <-q:
			l = append(l, items...)
		case <-doneC:

			if !(doneN == todoN) {
				doneN++
			}

			if !(doneP == todoP) {
				doneP++
			}

			if doneP == todoP && doneN == todoN {
				goto out
			}

		case <-ctx.Done():
			errs = errors.Join(errs, ctx.Err())
			goto out
		}
	}

out:

	output.Renderer{
		DefaultOutput: "tab=OBJECT:meta.object,NODE:meta.node,ACTION:data.action,KEY:data.key,LAST_RUN_AT:data.last_run_at,NEXT_RUN_AT:data.next_run_at,SCHEDULE:data.schedule",
		Output:        t.Output,
		Color:         t.Color,
		Data:          api.ScheduleList{Items: l, Kind: "ScheduleList"},
		Colorize:      rawconfig.Colorize,
	}.Print()
	return errs
}
