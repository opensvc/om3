package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/objectaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectUnset is the cobra flag set of the set command.
	CmdObjectUnset struct {
		object.OptsUnset
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectUnset) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectUnset) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "unset",
		Short: "Unset a configuration key",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectUnset) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithRemoteAction("unset"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"kw": t.Keywords,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			return nil, object.NewConfigurerFromPath(p).Unset(t.OptsUnset)
		}),
	).Do()
}
