package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints"
)

type (
	CmdObjectLs struct {
		flagSetGlobal
		flagSetObject
		NodeSelector string
		Local        bool
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectLs) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	t.flagSetGlobal.init(cmd)
	t.flagSetObject.init(cmd)
	cmd.Flags().BoolVarP(&t.Local, "local", "", false, "Report only local instances")
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
		ObjectSelector: mergeSelector(*selector, t.ObjectSelector, kind, "**"),
		Format:         t.Format,
		Color:          t.Color,
	}.Do()
}
