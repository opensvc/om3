package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/core/clientcontext"
)

type (
	ContextUserRemoveCmd struct {
		Name  string
		Force bool
	}
)

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
