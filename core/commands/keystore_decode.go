package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdKeystoreDecode is the cobra flag set of the decode command.
	CmdKeystoreDecode struct {
		object.OptsDecode
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdKeystoreDecode) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsDecode)
}

func (t *CmdKeystoreDecode) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "decode",
		Short: "decode a keystore object key value",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdKeystoreDecode) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithRemoteAction("keys"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"key": t.Key,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			return object.NewFromPath(p).(object.Keystorer).Decode(t.OptsDecode)
		}),
	).Do()
}
