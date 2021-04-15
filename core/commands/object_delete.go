package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/objectaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectSet is the cobra flag set of the set command.
	CmdObjectDelete struct {
		object.OptsDelete
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectDelete) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsDelete)
}

func (t *CmdObjectDelete) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "delete the object, an instance or a configuration section",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectDelete) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithRemoteAction("delete"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"unprovision": t.Unprovision,
			"rid":         t.ResourceSelector,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			return nil, object.NewConfigurerFromPath(p).Delete(t.OptsDelete)
		}),
	).Do()
}
