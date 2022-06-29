package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectEnter is the cobra flag set of the get command.
	CmdObjectEnter struct {
		object.OptsEnter
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectEnter) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectEnter) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "enter",
		Short: "open a shell in a container resource",
		Long:  "Enter any container resource if --rid is not set.",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectEnter) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewFromPath(p)
			if err != nil {
				return nil, err
			}
			c, ok := o.(object.Enterer)
			if !ok {
				return nil, fmt.Errorf("%s is not a enterer", o)
			}
			return nil, c.Enter(t.OptsEnter)
		}),
	).Do()
}
