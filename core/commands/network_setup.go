package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/network"
	"opensvc.com/opensvc/core/object"
)

type (
	// NetworkSetup is the cobra flag set of the command.
	NetworkSetup struct {
		Global object.OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NetworkSetup) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NetworkSetup) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "setup",
		Short:   "configure the cluster networks on the node",
		Aliases: []string{"set"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NetworkSetup) run() {
	if t.Global.Local || !clientcontext.IsSet() {
		t.doLocal()
	} else {
		t.doDaemon()
	}
}

func (t *NetworkSetup) doLocal() {
	n := object.NewNode()
	if err := network.Setup(n); err != nil {
		os.Exit(1)
	}
}

func (t *NetworkSetup) doDaemon() {
	var (
		c   *client.T
		err error
	)
	if c, err = client.New(client.WithURL(t.Global.Server)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	panic("TODO")
	fmt.Println(c)
}
