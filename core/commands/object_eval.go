package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/key"
)

type (
	// CmdObjectEval is the cobra flag set of the get command.
	CmdObjectEval struct {
		OptsGlobal
		Keyword     string `flag:"kw"`
		Impersonate string `flag:"impersonate"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectEval) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
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
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("eval"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"kw":          t.Keyword,
			"impersonate": t.Impersonate,
			"eval":        true,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			c, err := object.NewConfigurer(p)
			if err != nil {
				return nil, err
			}
			return c.EvalAs(key.Parse(t.Keyword), t.Impersonate)
		}),
	).Do()
}
