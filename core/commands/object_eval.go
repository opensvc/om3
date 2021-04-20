package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/objectaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectEval is the cobra flag set of the get command.
	CmdObjectEval struct {
		object.OptsEval
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectEval) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsEval)
}

func (t *CmdObjectEval) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "eval",
		Short: "evaluate a configuration key value",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectEval) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithRemoteAction("get"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"kw":          t.Keyword,
			"impersonate": t.Impersonate,
			"eval":        true,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			return object.NewFromPath(p).(object.Configurer).Eval(t.OptsEval)
		}),
	).Do()
}
