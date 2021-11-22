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
	CmdObjectSet struct {
		object.OptsSet
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectSet) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectSet) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "set",
		Short: "set a configuration key value",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectSet) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithRemoteAction("set"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"kw": t.KeywordOps,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewConfigurerFromPath(p)
			if err != nil {
				return nil, err
			}
			return nil, o.Set(t.OptsSet)
		}),
	).Do()
}
