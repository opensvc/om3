package commoncmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/util/duration"
)

type (
	ContextChangeCmd struct {
		Name                 string
		User                 string
		Cluster              string
		Namespace            string
		AccessTokenDuration  time.Duration
		RefreshTokenDuration time.Duration
	}
)

func NewCmdContextChange() *cobra.Command {
	var options ContextChangeCmd

	cmd := &cobra.Command{
		Use:   "change",
		Short: "change a context",
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

func (t *ContextChangeCmd) Run() error {

	configs, err := clientcontext.Load()
	if err != nil {
		return err
	}

	ctx, ok := configs.Contexts[t.Name]
	if !ok {
		return fmt.Errorf("context %s does not exist", t.Name)
	}

	ctx.UserRefName = t.User
	ctx.ClusterRefName = t.Cluster

	if t.Namespace != "" {
		ctx.Namespace = &t.Namespace
	}
	if t.AccessTokenDuration != 0 {
		ctx.AccessTokenDuration = duration.New(t.AccessTokenDuration)
	}

	if t.RefreshTokenDuration != 0 {
		ctx.RefreshTokenDuration = duration.New(t.RefreshTokenDuration)
	}

	if err := configs.ChangeContext(t.Name, ctx); err != nil {
		return err
	}

	if err := configs.Save(); err != nil {
		return err
	}

	return nil
}
