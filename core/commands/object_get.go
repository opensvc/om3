package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/objectaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectGet is the cobra flag set of the get command.
	CmdObjectGet struct {
		object.OptsGet
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectGet) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	object.InstallFlags(cmd, &t.OptsGet)
}

func (t *CmdObjectGet) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Get a configuration key value.",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectGet) run(selector *string, kind string) {
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
			"eval":        t.Eval,
		}),
		objectaction.WithLocalRun(func(path object.Path) (interface{}, error) {
			return path.NewObject().(object.Configurer).Get(t.OptsGet)
		}),
	).Do()
}
