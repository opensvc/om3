package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/xerrors"
)

type (
	CmdObjectPrintStatus struct {
		OptsGlobal
		OptsLock
		Refresh bool
	}
)

func (t *CmdObjectPrintStatus) extract(selector string, c *client.T) ([]object.Status, error) {
	if t.Refresh || t.Local {
		// explicitely local
		return t.extractLocal(selector)
	}
	if data, err := t.extractFromDaemon(selector, c); err == nil {
		// try daemon
		return data, nil
	} else if clientcontext.IsSet() {
		// no fallback for remote cluster
		return []object.Status{}, err
	}
	// fallback to local
	return t.extractLocal(selector)
}

func (t *CmdObjectPrintStatus) extractLocal(selector string) ([]object.Status, error) {
	data := make([]object.Status, 0)
	sel := objectselector.NewSelection(
		selector,
		objectselector.SelectionWithLocal(true),
	)
	h := hostname.Hostname()
	paths, err := sel.Expand()
	if err != nil {
		return data, err
	}
	n, err := object.NewNode()
	if err != nil {
		return data, err
	}

	var errs error
	ctx := context.Background()
	ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
	ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
	for _, p := range paths {
		obj, err := object.NewCore(p)
		if err != nil {
			errs = xerrors.Append(errs, err)
			continue
		}
		var status instance.Status
		if t.Refresh {
			status, err = obj.FreshStatus(ctx)
		} else {
			status, err = obj.Status(ctx)
		}
		if err != nil {
			errs = xerrors.Append(errs, err)
			continue
		}
		o := object.Status{
			Path:   p,
			Compat: true,
			Object: object.AggregatedStatus{},
			Instances: map[string]instance.States{
				h: {
					Node: instance.Node{
						Name:   h,
						Frozen: n.Frozen(),
					},
					Status: status,
				},
			},
		}
		data = append(data, o)
	}
	return data, errs
}

func (t *CmdObjectPrintStatus) extractFromDaemon(selector string, c *client.T) ([]object.Status, error) {
	var (
		err           error
		b             []byte
		clusterStatus cluster.Status
	)
	b, err = c.NewGetDaemonStatus().
		SetSelector(selector).
		SetRelatives(true).
		Do()
	if err != nil {
		return []object.Status{}, err
	}
	err = json.Unmarshal(b, &clusterStatus)
	if err != nil {
		return []object.Status{}, err
	}
	data := make([]object.Status, 0)
	for ps := range clusterStatus.Cluster.Object {
		p, err := path.Parse(ps)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", p, err)
			continue
		}
		data = append(data, clusterStatus.GetObjectStatus(p))
	}
	return data, nil
}

func (t *CmdObjectPrintStatus) Run(selector, kind string) error {
	var (
		data []object.Status
		err  error
	)
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	sel := objectselector.NewSelection(
		mergedSelector,
		objectselector.SelectionWithClient(c),
	)
	paths, err := sel.ExpandSet()
	if err != nil {
		return errors.Wrap(err, "expand selection")
	}
	data, err = t.extract(mergedSelector, c)
	if err != nil {
		return errors.Wrap(err, "extract data")
	}

	output.Renderer{
		Format: t.Format,
		Color:  t.Color,
		Data:   data,
		HumanRenderer: func() string {
			s := ""
			for _, d := range data {
				if !paths.Has(d.Path) {
					continue
				}
				s += d.Render()
			}
			return s
		},
		Colorize: rawconfig.Colorize,
	}.Print()
	return nil
}
