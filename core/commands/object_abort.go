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
	// CmdObjectAbort is the cobra flag set of the abort command.
	CmdObjectAbort struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectAbort) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectAbort) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "abort",
		Short: "abort the running orchestration",
		Run: func(cmd *cobra.Command, args []string) {
			if err := t.run(selector, kind); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}
}

func (t *CmdObjectAbort) run(selector *string, kind string) error {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	sel := objectselector.NewSelection(mergedSelector)
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	var errs error
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	req := c.NewPostObjectAbort()
	for _, p := range paths {
		req.Path = p
		if _, err := req.Do(); err != nil {
			errs = xerrors.Append(errs, err)
			break // no need to post on every node
		}
	}
	return errs
}
