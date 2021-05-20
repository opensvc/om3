package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdKeystoreChange is the cobra flag set of the decode command.
	CmdKeystoreChange struct {
		object.OptsAdd
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdKeystoreChange) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdKeystoreChange) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "change",
		Short: "change existing keys value",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdKeystoreChange) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithRemoteAction("change"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"key":   t.Key,
			"from":  t.From,
			"value": t.Value,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			return nil, object.NewFromPath(p).(object.Keystorer).Change(t.OptsAdd)
		}),
	).Do()
}
