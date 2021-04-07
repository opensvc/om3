package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/objectaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectStop is the cobra flag set of the stop command.
	CmdObjectStop struct {
		object.OptsStop
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectStop) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectStop) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the selected objects",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectStop) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithRemoteAction("stop"),
		objectaction.WithAsyncTarget("stopped"),
		objectaction.WithAsyncWatch(t.Async.Watch),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			intf := object.NewFromPath(p).(object.Starter)
			return nil, intf.Stop(t.OptsStop)
		}),
	).Do()
}
