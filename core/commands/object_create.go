package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectCreate is the cobra flag set of the create command.
	CmdObjectCreate struct {
		object.OptsCreate
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectCreate) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectCreate) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "create new objects",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectCreate) run(selector *string, kind string) {
}
