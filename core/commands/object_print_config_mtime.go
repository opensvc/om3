package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	// CmdObjectPrintConfigMtime is the cobra flag set of the print config command.
	CmdObjectPrintConfigMtime struct {
		object.OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectPrintConfigMtime) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectPrintConfigMtime) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "mtime",
		Short:   "Print selected object and instance configuration file modification time",
		Aliases: []string{"mtim", "mti", "mt", "m"},
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectPrintConfigMtime) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.OptsGlobal.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.OptsGlobal.Local),
		objectaction.WithFormat(t.OptsGlobal.Format),
		objectaction.WithColor(t.OptsGlobal.Color),
		objectaction.WithRemoteNodes(t.OptsGlobal.NodeSelector),
		objectaction.WithRemoteAction("print_config_mtime"),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			tm := object.NewFromPath(p).(object.Configurer).Config().ModTime()
			return timestamp.New(tm).String(), nil
		}),
	).Do()
}
