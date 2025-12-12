package oxcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	CmdObjectInstanceStatus struct {
		OptsGlobal
		commoncmd.OptsLock
		NodeSelector string
		Refresh      bool
	}
)

func (t *CmdObjectInstanceStatus) extract(paths naming.Paths, c *client.T) ([]object.Digest, error) {
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
	if clientcontext.IsSet() {
		if nodes, err := nodeselector.New("*", nodeselector.WithClient(c)).Expand(); err != nil {
			return nil, fmt.Errorf("expand node selection: %w", err)
		} else {
			return nodes, nil
		}
	}
	return []string{}, nil
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
	data, err = t.extract(paths, c)
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
