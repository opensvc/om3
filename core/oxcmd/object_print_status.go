package oxcmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
)

type (
	CmdObjectPrintStatus struct {
		OptsGlobal
		OptsLock
		NodeSelector string
		Refresh      bool
	}
)

func (t *CmdObjectPrintStatus) extract(selector string, c *client.T) ([]object.Digest, error) {
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

func (t *CmdObjectPrintStatus) getNodenames(c *client.T) ([]string, error) {
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

func (t *CmdObjectPrintStatus) Run(selector, kind string) error {
	var (
		data []object.Digest
		err  error
	)
	if t.Refresh {
		return fmt.Errorf("todo: honor --refresh")
	}
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	sel := objectselector.New(
		mergedSelector,
		objectselector.WithClient(c),
	)
	paths, err := sel.ExpandSet()
	if err != nil {
		return fmt.Errorf("expand object selection: %w", err)
	}
	nodenames, err := t.getNodenames(c)
	if err != nil {
		return err
	}
	data, err = t.extract(mergedSelector, c)
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
				if !paths.Contains(d.Path) {
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
