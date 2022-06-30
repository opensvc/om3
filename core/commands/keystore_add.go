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
		OptsGlobal
		object.OptsAdd
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdKeystoreAdd) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdKeystoreAdd) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "add new keys",
		Run: func(cmd *cobra.Command, args []string) {
			if !cmd.Flags().Changed("value") {
				t.Value = nil
			}
			if !cmd.Flags().Changed("from") {
				t.From = nil
			}
			t.run(selector, kind)
		},
	}
}

func (t *CmdKeystoreAdd) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("add"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"key":   t.Key,
			"from":  t.From,
			"value": t.Value,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			store, err := object.NewKeystore(p)
			if err != nil {
				return nil, err
			}
			return nil, store.Add(t.OptsAdd)
		}),
	).Do()
}
