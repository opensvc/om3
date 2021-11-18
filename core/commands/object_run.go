package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectRun is the cobra flag set of the run command.
	CmdObjectRun struct {
		object.OptsRun
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectRun) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectRun) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "run tasks now",
		Long:  "The svc and vol objects can define task resources. Tasks are usually run on a schedule, but this command can trigger a run now.",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectRun) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.OptsGlobal.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithResourceSelectorOptions(t.OptsResourceSelector.Options),
		objectaction.WithLocal(t.OptsGlobal.Local),
		objectaction.WithFormat(t.OptsGlobal.Format),
		objectaction.WithColor(t.OptsGlobal.Color),
		objectaction.WithRemoteNodes(t.OptsGlobal.NodeSelector),
		objectaction.WithRemoteAction("run"),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewActorFromPath(p)
			if err != nil {
				return nil, err
			}
			return nil, o.Run(t.OptsRun)
		}),
	).Do()
}
