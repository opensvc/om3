package oxcmd

import (
	"github.com/opensvc/om3/core/clientcontext"
)

type (
	ContextUserAddCmd struct {
		Name              string
		Username          string
		ClientKey         string
		ClientCertificate string
	}
)

func (t *ContextUserAddCmd) Run() error {
	user := clientcontext.User{}

	if t.Username != "" {
		user.Name = &t.Username
	}

	if t.ClientKey != "" {
		user.ClientKey = &t.ClientKey
	}

	if t.ClientCertificate != "" {
		user.ClientCertificate = &t.ClientCertificate
	}

	cfg, err := clientcontext.Load()
	if err != nil {
		return err
	}

	if err := cfg.AddUser(t.Name, user); err != nil {
		return err
	}

	return cfg.Save()
}
