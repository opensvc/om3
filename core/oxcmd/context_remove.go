package oxcmd

import (
	"github.com/opensvc/om3/v3/core/clientcontext"
)

type (
	ContextRemoveCmd struct {
		Name string
	}
)

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
