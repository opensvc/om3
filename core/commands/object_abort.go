package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/monitor"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectAbort struct {
		OptsAsync
		OptsGlobal
	}
)

func (t *CmdObjectAbort) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	sel := objectselector.NewSelection(mergedSelector)
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	var errs error
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	params := api.PostObjectAbort{}
	for _, p := range paths {
		params.Path = p.String()
		if resp, err := c.PostObjectAbort(context.Background(), params); err != nil {
			errs = errors.Join(errs, err)
		} else if resp.StatusCode != http.StatusOK {
			errs = errors.Join(errs, fmt.Errorf("unexpected post object abort status code %s", resp.Status))
		}
	}
	if t.Watch {
		m := monitor.New()
		m.SetColor(t.Color)
		m.SetFormat(t.Output)
		m.SetSelector(mergedSelector)
		cli, e := client.New(client.WithURL(t.Server), client.WithTimeout(0))
		if e != nil {
			_, _ = fmt.Fprintln(os.Stderr, e)
			return e
		}
		statusGetter := cli.NewGetDaemonStatus().SetSelector(mergedSelector)
		evReader, err := cli.NewGetEvents().SetSelector(mergedSelector).GetReader()
		errs = errors.Join(errs, err)
		err = m.DoWatch(statusGetter, evReader, os.Stdout)
		errs = errors.Join(errs, err)
	}
	return errs
}
