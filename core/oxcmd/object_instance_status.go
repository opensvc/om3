package oxcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
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
	"github.com/opensvc/om3/v3/core/statusboard"
)

type (
	CmdObjectInstanceStatus struct {
		OptsGlobal
		commoncmd.OptsLock
		NodeSelector string
		Refresh      bool
	}
)

// extract returns the statuses of the objects, refreshed by every node first
// with --refresh. A refresh failing on some nodes does not prevent the
// statuses from being returned, with the last known of these nodes, and
// refreshErr names them.
func (t *CmdObjectInstanceStatus) extract(paths naming.Paths, c *client.T) (data []object.Digest, refreshErr error, err error) {
	var clusterStatus clusterdump.Data
	getClusterStatus := func(selector string) error {
		b, err := c.NewGetClusterStatus().
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
		return nil, nil, err
	}

	if t.Refresh {
		refreshCtx, cancel := context.WithTimeout(ctx, commoncmd.StatusRefreshTimeout)
		refreshErr = commoncmd.RefreshInstanceStatusFromClusterStatus(refreshCtx, clusterStatus)
		cancel()
		if err := getClusterStatus(selector); err != nil {
			return nil, refreshErr, err
		}
	}

	data = make([]object.Digest, 0)
	for ps := range clusterStatus.Cluster.Object {
		p, err := naming.ParsePath(ps)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", p, err)
			continue
		}
		data = append(data, clusterStatus.GetObjectStatus(p))
	}
	return data, refreshErr, nil
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
	data, refreshErr, err := t.extract(paths, c)
	if err != nil {
		return errors.Join(refreshErr, err)
	}
	renderer := output.Renderer{
		Output: t.Output,
		Sort:   t.Sort,
		Color:  t.Color,
		Data:   data,
		HumanRenderer: func() string {
			// In the order of the paths, separated as the status boards
			// are.
			shown := make([]object.Digest, 0, len(data))
			for _, d := range data {
				if pathMap.HasPath(d.Path) {
					shown = append(shown, d)
				}
			}
			sort.Slice(shown, func(i, j int) bool { return shown[i].Path.String() < shown[j].Path.String() })
			l := make([]string, len(shown))
			for i, d := range shown {
				l[i] = d.Render(nodenames)
			}
			return statusboard.JoinDocuments(l)
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
	if err := renderer.Print(); err != nil {
		return err
	}
	return refreshErr
}
