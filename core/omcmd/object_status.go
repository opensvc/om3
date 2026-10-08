package omcmd

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/statusboard"
)

type (
	// CmdObjectStatus shows the status of the objects cluster-wide: the
	// board of their resources by node, and the notes saying what needs
	// attention.
	CmdObjectStatus struct {
		OptsGlobal
		commoncmd.OptsLock
		Refresh bool
		Monitor bool
	}
)

func (t *CmdObjectStatus) Run(kind string) error {
	// The objects selected, or all the objects of the kind.
	defaultSelector := "**"
	if kind != "" {
		defaultSelector = "*/" + kind + "/*"
	}
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, defaultSelector)
	c, err := client.New()
	if err != nil {
		return err
	}
	sel := objectselector.New(
		mergedSelector,
		objectselector.WithClient(c),
		objectselector.WithLocal(true),
	)
	paths, err := sel.MustExpand()
	if err != nil {
		return fmt.Errorf("expand object selection: %w", err)
	}
	pathMap := paths.StrMap()

	// With --refresh, every node evaluates again the status of its
	// instances, and the board waits for them. Without a daemon to ask the
	// nodes, the local instance alone is evaluated again.
	var refreshErr error
	refresh := t.Refresh
	if t.Refresh {
		clusterStatus, err := getClusterStatus(paths, c)
		switch {
		case err == nil:
			ctx, cancel := context.WithTimeout(context.Background(), commoncmd.StatusRefreshTimeout)
			refreshErr = commoncmd.RefreshInstanceStatusFromClusterStatus(ctx, clusterStatus)
			cancel()
			refresh = false
		case client.IsDaemonDown(err):
			// No daemon: the local instance is refreshed alone.
		default:
			return fmt.Errorf("refresh: %w", err)
		}
	}

	// The instance status command gathers the same dataset: the instances
	// of every node the daemon knows, or the local instance alone when the
	// daemon is not running.
	gather := CmdObjectInstanceStatus{
		OptsGlobal: t.OptsGlobal,
		OptsLock:   t.OptsLock,
		Refresh:    refresh,
		Monitor:    t.Monitor,
	}
	data, err := gather.extract(nil, paths, c)
	if err != nil {
		return errors.Join(refreshErr, err)
	}
	shown := make([]object.Digest, 0, len(data))
	for _, d := range data {
		if pathMap.HasPath(d.Path) {
			shown = append(shown, d)
		}
	}
	// In the order of the paths, the daemon answering them in no order.
	sort.Slice(shown, func(i, j int) bool { return shown[i].Path.String() < shown[j].Path.String() })
	renderer := output.Renderer{
		Output: t.Output,
		Color:  t.Color,
		Data:   shown,
		HumanRenderer: func() string {
			return statusboard.RenderTerminalList(shown)
		},
		Colorize: rawconfig.Colorize,
	}
	if err := renderer.Print(); err != nil {
		return err
	}
	// The board shows the statuses the nodes refreshed, and the last known
	// of the others, which the error names.
	return refreshErr
}
