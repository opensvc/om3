package commoncmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/clientcontext"
)

type (
	ContextClusterRemoveCmd struct {
		Name  string
		Force bool
	}
)

func NewCmdContextClusterRemove() *cobra.Command {
	var options ContextClusterRemoveCmd

	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Short:   "remove a cluster context endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&options.Name, "name", "", "Cluster name")
	flags.BoolVar(&options.Force, "force", false, "Force removal even if the cluster is used in a context")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func (t *ContextClusterRemoveCmd) Run() error {
	cfg, err := clientcontext.Load()
	if err != nil {
		return err
	}
	if !t.Force && cfg.ClusterUsed(t.Name) {
		return fmt.Errorf("cluster %s is used in one or more contexts, use --force to remove it anyway", t.Name)
	}
	err = cfg.RemoveCluster(t.Name)
	if err != nil {
		return err
	}
	return cfg.Save()
}
