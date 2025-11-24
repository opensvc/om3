package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/core/clientcontext"
)

type (
	ContextUserChangeCmd struct {
		Name              string
		Username          string
		ClientKey         string
		ClientCertificate string
	}
)

func (t *ContextUserChangeCmd) Run() error {
	configs, err := clientcontext.Load()
	if err != nil {
		return err
	}

	usr, ok := configs.Users[t.Name]
	if !ok {
		return fmt.Errorf("user %s does not exist", t.Name)
	}

	if t.Username != "" {
		usr.Name = &t.Username
	}

	if t.ClientKey != "" {
		usr.ClientKey = &t.ClientKey
	}

	if t.ClientCertificate != "" {
		usr.ClientCertificate = &t.ClientCertificate
	}

	if err := configs.ChangeUser(t.Name, usr); err != nil {
		return err
	}

	return configs.Save()
}
