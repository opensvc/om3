package oxcmd

import (
	"fmt"
	"time"

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
