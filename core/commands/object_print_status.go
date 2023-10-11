package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectlogger"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdObjectPrintStatus struct {
		OptsGlobal
		OptsLock
		Refresh bool
	}
)

func (t *CmdObjectPrintStatus) extract(selector string, c *client.T) (data []object.Digest, err error) {
	var errs error
	if t.Refresh || t.Local {
		// explicitely local
		if data, err = t.extractLocal(selector); err != nil {
			errs = errors.Join(errs, err)
		}

	}
	if data, err := t.extractFromDaemon(selector, c); err == nil {
		// try daemon
		return data, errs
	} else {
		errs = errors.Join(errs, err)
		if clientcontext.IsSet() {
			// no fallback for remote cluster
			return []object.Digest{}, errs
		}
	}
	// fallback to local
	if data != nil {
		return data, errs
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
		logger := objectlogger.New(p,
			objectlogger.WithColor(t.Color != "no"),
			objectlogger.WithConsoleLog(t.Log != ""),
			objectlogger.WithLogFile(true),
		)
		obj, err := object.NewCore(p, object.WithLogger(logger))
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		var status instance.Status
		if t.Refresh {
			status, err = obj.FreshStatus(ctx)
		} else {
			status, err = obj.Status(ctx)
		}
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", p, err))
			continue
		}
		o := object.Digest{
			Path:     p,
			IsCompat: true,
			Object:   object.Status{},
			Instances: []instance.States{
				{
					Node: instance.Node{
						Name:     h,
						FrozenAt: n.Frozen(),
					},
					Path:   p,
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
		Get()
	if err != nil {
		return []object.Digest{}, err
	}
	err = json.Unmarshal(b, &clusterStatus)
	if err != nil {
		return []object.Digest{}, err
	}
	data := make([]object.Digest, 0)
	for ps := range clusterStatus.Cluster.Object {
		p, err := naming.ParsePath(ps)
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
		return fmt.Errorf("expand object selection: %w", err)
	}
	data, _ = t.extract(mergedSelector, c)
	renderer := output.Renderer{
		Output: t.Output,
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
	}
	if t.NodeSelector != "" {
		sel := nodeselector.New(
			t.NodeSelector,
			nodeselector.WithClient(c),
		)
		nodes, err := sel.Expand()
		if err != nil {
			return fmt.Errorf("expand node selection: %w", err)
		}
		l := make([]instance.States, 0)
		for _, objData := range data {
			instMap := objData.Instances.ByNode()
			for _, node := range nodes {
				if _, ok := instMap[node]; !ok {
					return fmt.Errorf("instance of %s on node %s does not exist", objData.Path, node)
				}
				l = append(l, instMap[node])
			}
		}
		renderer.Data = l
	}
	renderer.Print()
	return nil
}
