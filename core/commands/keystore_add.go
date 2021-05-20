package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdKeystoreAdd is the cobra flag set of the decode command.
	CmdKeystoreAdd struct {
		object.OptsAdd
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdKeystoreAdd) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsAdd)
}

func (t *CmdKeystoreAdd) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "add new keys",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdKeystoreAdd) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithRemoteAction("add"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"key":   t.Key,
			"from":  t.From,
			"value": t.Value,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			return nil, object.NewFromPath(p).(object.Keystorer).Add(t.OptsAdd)
		}),
	).Do()
}
