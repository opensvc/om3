package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/util/xerrors"
)

type (
	// CmdObjectClear is the cobra flag set of the clear command.
	CmdObjectClear struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectClear) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectClear) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "clear errors in the monitor state",
		Run: func(cmd *cobra.Command, args []string) {
			if err := t.run(selector, kind); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}
}

func (t *CmdObjectClear) run(selector *string, kind string) error {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	sel := objectselector.NewSelection(mergedSelector)
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	var errs error
	for _, p := range paths {
		for _, node := range nodesFromPath(p) {
			c, err := client.New(
				client.WithURL(node),
			)
			if err != nil {
				return err
			}
			req := c.NewPostObjectClear()
			req.Path = p
			if _, err := req.Do(); err != nil {
				errs = xerrors.Append(errs, err)
			}
		}
	}
	return errs
}
