package omcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	CmdNodeRegister struct {
		OptsGlobal
		CredentialFile string
		User           string
		Password       string
		App            string
		NodeSelector   string
	}
)

func (t *CmdNodeRegister) Run() error {
	user, password, err := commoncmd.CollectorCredential(t.CredentialFile)
	if err != nil {
		return err
	}
	if user == "" && t.User != "" {
		// The deprecated --user and --password, kept for the commands
		// written against the previous release. A credential wins over
		// them: an operator naming one is asking for it to be used.
		user, password = t.User, t.Password
	}
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
			return nil, n.Register(ctx, user, password, t.App)
		}),
	).Do()
}
