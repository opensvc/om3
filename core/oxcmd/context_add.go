package oxcmd

import (
	"time"

	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/util/duration"
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

func (t *ContextAddCmd) Run() error {

	relation := clientcontext.Relation{
		ClusterRefName: t.Cluster,
		UserRefName:    t.User,
	}

	if t.Namespace != "" {
		relation.Namespace = &t.Namespace
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
