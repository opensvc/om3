package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdKeystoreInstall is the cobra flag set of the decode command.
	CmdKeystoreInstall struct {
		object.OptsInstall
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdKeystoreInstall) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsInstall)
}

func (t *CmdKeystoreInstall) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "install keys as files in their projected volume locations",
		Long:  "Keys of sec and cfg can be projected to volumes via the configs and secrets keywords of volume resources. When a key value change all projections are automatically refreshed. This command triggers manually the same operations.",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdKeystoreInstall) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithRemoteAction("install"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"key": t.Key,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			return nil, object.NewFromPath(p).(object.Keystorer).Install(t.OptsInstall)
		}),
	).Do()
}
