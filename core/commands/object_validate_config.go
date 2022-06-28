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
	// CmdObjectValidateConfig is the cobra flag set of the get command.
	CmdObjectValidateConfig struct {
		object.OptsValidateConfig
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectValidateConfig) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsValidateConfig)
}

func (t *CmdObjectValidateConfig) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "config",
		Short:   "verify the object configuration syntax is correct",
		Aliases: []string{"confi", "conf", "con", "co", "c"},
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectValidateConfig) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("validate config"),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewFromPath(p)
			if err != nil {
				return nil, err
			}
			c, ok := o.(object.Configurer)
			if !ok {
				return nil, fmt.Errorf("%s is not a configurer", o)
			}
			return c.ValidateConfig(t.OptsValidateConfig)
		}),
	).Do()
}
