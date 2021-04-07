package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectLs is the cobra flag set of the ls command.
	CmdObjectLs struct {
		Global object.OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectLs) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectLs) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "Print the selected objects path",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectLs) run(selector *string, kind string) {
	entrypoints.List{
		ObjectSelector: mergeSelector(*selector, t.Global.ObjectSelector, kind, "**"),
		Format:         t.Global.Format,
		Color:          t.Global.Color,
		Local:          t.Global.Local,
		Server:         t.Global.Server,
	}.Do()
}
