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
	// CmdObjectFreeze is the cobra flag set of the freeze command.
	CmdObjectFreeze struct {
		OptsGlobal
		OptsAsync
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectFreeze) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectFreeze) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "freeze",
		Short: "freeze the selected objects",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectFreeze) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.WithLocal(t.Local),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithFormat(t.Format),
		objectaction.WithColor(t.Color),
		objectaction.WithServer(t.Server),
		objectaction.WithAsyncTarget("frozen"),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("freeze"),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			return nil, o.Freeze(context.Background())
		}),
	).Do()
}
