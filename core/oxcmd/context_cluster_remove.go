package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/clientcontext"
)

type (
	ContextClusterRemoveCmd struct {
		Name  string
		Force bool
	}
)

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
