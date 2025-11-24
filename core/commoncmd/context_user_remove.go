package commoncmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/clientcontext"
)

type (
	ContextUserRemoveCmd struct {
		Name  string
		Force bool
	}
)

func NewCmdContextUserRemove() *cobra.Command {
	var options ContextUserRemoveCmd

	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Short:   "remove a context user",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&options.Name, "name", "", "User name")
	flags.BoolVar(&options.Force, "force", false, "Force removal even if the user is used in a context")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func (t *ContextUserRemoveCmd) Run() error {
	cfg, err := clientcontext.Load()
	if err != nil {
		return err
	}

	if !t.Force && cfg.UserUsed(t.Name) {
		return fmt.Errorf("user %s is used in one or more contexts, use --force to remove it anyway", t.Name)
	}

	err = cfg.RemoveUser(t.Name)
	if err != nil {
		return err
	}

	return cfg.Save()
}
