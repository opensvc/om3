package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectSet is the cobra flag set of the set command.
	CmdObjectDelete struct {
		OptsGlobal
		object.OptsDelete
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectDelete) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
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
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("delete"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"unprovision": t.Unprovision,
			"rid":         t.RID,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewConfigurerFromPath(p)
			if err != nil {
				return nil, err
			}
			return nil, o.Delete(t.OptsDelete)
		}),
	).Do()
}
