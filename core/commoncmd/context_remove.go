package commoncmd

import (
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/clientcontext"
)

type (
	ContextRemoveCmd struct {
		Name string
	}
)

func NewCmdContextRemove() *cobra.Command {
	var options ContextRemoveCmd

	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Short:   "remove an existing context",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&options.Name, "name", "", "Context name")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func (t *ContextRemoveCmd) Run() error {
	cfg, err := clientcontext.Load()
	if err != nil {
		return err
	}
	err = cfg.RemoveContext(t.Name)
	if err != nil {
		return err
	}
	return cfg.Save()
}
