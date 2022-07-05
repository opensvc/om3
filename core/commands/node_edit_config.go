package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectEditConfig is the cobra flag set of the print config command.
	NodeEditConfig struct {
		OptsGlobal
		Discard bool `flag:"discard"`
		Recover bool `flag:"recover"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeEditConfig) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
	cmd.MarkFlagsMutuallyExclusive("discard", "recover")
}

func (t *NodeEditConfig) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "config",
		Short:   "edit the node configuration",
		Aliases: []string{"confi", "conf", "con", "co", "c", "cf", "cfg"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodeEditConfig) run() {
	var err error
	switch {
	//case t.Discard && t.Recover:
	//        return errors.New("discard and recover options are mutually exclusive")
	case t.Discard:
		err = object.NewNode().DiscardAndEditConfig()
	case t.Recover:
		err = object.NewNode().RecoverAndEditConfig()
	default:
		err = object.NewNode().EditConfig()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
