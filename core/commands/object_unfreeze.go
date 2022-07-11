package commands

import (
	"context"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectUnfreeze is the cobra flag set of the unfreeze command.
	CmdObjectUnfreeze struct {
		OptsGlobal
		OptsAsync
	}
	CmdObjectThaw struct {
		CmdObjectUnfreeze
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectUnfreeze) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectThaw) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectUnfreeze) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "unfreeze",
		Short: "unfreeze the selected objects",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectThaw) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:        "thaw",
		Deprecated: "use \"unfreeze\"",
		Short:      "unfreeze the selected objects",
		Run: func(cmd *cobra.Command, args []string) {
			t.CmdObjectUnfreeze.run(selector, kind)
		},
	}
}

func (t *CmdObjectUnfreeze) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.WithLocal(t.Local),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithFormat(t.Format),
		objectaction.WithColor(t.Color),
		objectaction.WithServer(t.Server),
		objectaction.WithAsyncTarget("thawed"),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("unfreeze"),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			return nil, o.Unfreeze(context.Background())
		}),
	).Do()
}
