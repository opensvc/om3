package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/xerrors"
	"github.com/pkg/errors"
)

type (
	CmdObjectPrintStatus struct {
		OptsGlobal
		OptsLock
		Refresh bool
	}
)

func (t *CmdObjectPrintStatus) extract(selector string, c *client.T) (data []object.Digest, err error) {
	if t.Refresh || t.Local {
		// explicitely local
		data, err = t.extractLocal(selector)
	}
	if data, err := t.extractFromDaemon(selector, c); err == nil {
		// try daemon
		return data, nil
	} else if clientcontext.IsSet() {
		// no fallback for remote cluster
		return []object.Digest{}, err
	}
	// fallback to local
	if data != nil {
		return data, err
	}
	return t.extractLocal(selector)
}

func (t *CmdObjectPrintStatus) extractLocal(selector string) ([]object.Digest, error) {
	data := make([]object.Digest, 0)
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
		o := object.Digest{
			Path:   p,
			Compat: true,
			Object: object.Status{},
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

func (t *CmdObjectPrintStatus) extractFromDaemon(selector string, c *client.T) ([]object.Digest, error) {
	var (
		err           error
		b             []byte
		clusterStatus cluster.Data
	)
	b, err = c.NewGetDaemonStatus().
		SetSelector(selector).
		SetRelatives(true).
		Do()
	if err != nil {
		return []object.Digest{}, err
	}
	err = json.Unmarshal(b, &clusterStatus)
	if err != nil {
		return []object.Digest{}, err
	}
	data := make([]object.Digest, 0)
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
		data []object.Digest
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
				if !paths.Contains(d.Path) {
					continue
				}
				s += d.Render([]string{hostname.Hostname()})
			}
			return s
		},
		Colorize: rawconfig.Colorize,
	}.Print()
	return nil
}
