package omcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdObjectInstanceStatus struct {
		OptsGlobal
		commoncmd.OptsLock
		NodeSelector string
		Refresh      bool
		Local        bool
	}
)

func (t *CmdObjectInstanceStatus) extract(nodenames []string, paths naming.Paths, c *client.T) (data []object.Digest, err error) {
	var localData []object.Digest
	if t.Local || (t.Refresh && t.NodeSelector == "") {
		localData, err = t.extractLocal(paths)
		if err != nil {
			return
		}
	}

	// try daemon
	data, err = t.extractFromDaemon(paths, c)
	if err == nil {
		return
	}

	if localData != nil {
		return localData, nil
	}

	data, err = t.extractLocal(paths)
	return
}

func (t *CmdObjectInstanceStatus) extractLocal(paths naming.Paths) ([]object.Digest, error) {
	data := make([]object.Digest, 0)
	h := hostname.Hostname()
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

func (t *CmdObjectInstanceStatus) extractFromDaemon(paths naming.Paths, c *client.T) ([]object.Digest, error) {
	var (
		err           error
		b             []byte
		clusterStatus clusterdump.Data
	)
	getClusterStatus := func(selector string) error {
		b, err = c.NewGetClusterStatus().
			SetSelector(selector).
			Get()
		if err != nil {
			return err
		}
		err = json.Unmarshal(b, &clusterStatus)
		if err != nil {
			return err
		}
		return nil
	}

	ctx := context.Background()
	strSlice := make([]string, len(paths))
	for i, path := range paths {
		strSlice[i] = path.String()
	}
	selector := strings.Join(strSlice, ",")

	if err := getClusterStatus(selector); err != nil {
		return []object.Digest{}, err
	}

	if t.Refresh {
		if err := commoncmd.RefreshInstanceStatusFromClusterStatus(ctx, clusterStatus); err != nil {
			return []object.Digest{}, err
		}
		if err := getClusterStatus(selector); err != nil {
			return []object.Digest{}, err
		}
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

func (t *CmdObjectInstanceStatus) getNodenames(c *client.T) ([]string, error) {
	if t.NodeSelector != "" {
		if nodes, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand(); err != nil {
			return nil, fmt.Errorf("expand node selection: %w", err)
		} else {
			return nodes, nil
		}
	}
	return []string{hostname.Hostname()}, nil
}

func (t *CmdObjectInstanceStatus) Run(kind string) error {
	var (
		data []object.Digest
		err  error
	)
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	c, err := client.New()
	if err != nil {
		return err
	}
	sel := objectselector.New(
		mergedSelector,
		objectselector.WithClient(c),
	)
	paths, err := sel.MustExpand()
	if err != nil {
		return fmt.Errorf("expand object selection: %w", err)
	}
	pathMap := paths.StrMap()
	nodenames, err := t.getNodenames(c)
	if err != nil {
		return err
	}
	data, err = t.extract(nodenames, paths, c)
	if err != nil {
		return err
	}
	renderer := output.Renderer{
		Output: t.Output,
		Color:  t.Color,
		Data:   data,
		HumanRenderer: func() string {
			s := ""
			for _, d := range data {
				if !pathMap.HasPath(d.Path) {
					continue
				}
				s += d.Render(nodenames)
			}
			return s
		},
		Colorize: rawconfig.Colorize,
	}
	l := make([]instance.States, 0)
	for _, objData := range data {
		instMap := objData.Instances.ByNode()
		for _, nodename := range nodenames {
			if _, ok := instMap[nodename]; !ok {
				continue
			}
			l = append(l, instMap[nodename])
		}
	}
	renderer.Data = l
	renderer.Print()
	return nil
}
