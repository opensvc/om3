package commoncmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/util/duration"
)

type (
	ContextAddCmd struct {
		Name                 string
		User                 string
		Cluster              string
		Namespace            string
		AccessTokenDuration  time.Duration
		RefreshTokenDuration time.Duration
	}
)

func NewCmdContextAdd() *cobra.Command {
	var options ContextAddCmd

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new context",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&options.Name, "name", "", "Context name")
	flags.StringVar(&options.User, "user", "", "User name")
	flags.StringVar(&options.Cluster, "cluster", "", "Cluster name")
	flags.StringVar(&options.Namespace, "namespace", "", "Namespace")
	flags.DurationVar(&options.AccessTokenDuration, "access-token-duration", 0, "Access token duration")
	flags.DurationVar(&options.RefreshTokenDuration, "refresh-token-duration", 0, "Refresh token duration")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("user")
	_ = cmd.MarkFlagRequired("cluster")

	return cmd
}

func (t *ContextAddCmd) Run() error {

	relation := clientcontext.Relation{
		ClusterRefName: t.Cluster,
		UserRefName:    t.User,
	}

	if t.Namespace != "" {
		relation.Namespace = t.Namespace
	}
	if t.AccessTokenDuration != 0 {
		relation.AccessTokenDuration = duration.New(t.AccessTokenDuration)
	}
	if t.RefreshTokenDuration != 0 {
		relation.RefreshTokenDuration = duration.New(t.RefreshTokenDuration)
	}

	cfg, err := clientcontext.Load()
	if err != nil {
		return err
	}

	if err := cfg.AddContext(t.Name, relation); err != nil {
		return err
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	return nil
}
