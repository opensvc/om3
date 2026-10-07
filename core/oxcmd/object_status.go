package oxcmd

import (
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
		Refresh bool
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
	paths, err := objectselector.New(mergedSelector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		return fmt.Errorf("expand object selection: %w", err)
	}
	pathMap := paths.StrMap()

	// The instance status command gathers the same dataset from the api.
	gather := CmdObjectInstanceStatus{OptsGlobal: t.OptsGlobal, Refresh: t.Refresh}
	data, err := gather.extract(paths, c)
	if err != nil {
		return err
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
	return renderer.Print()
}
