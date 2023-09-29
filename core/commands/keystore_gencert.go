package commands

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/naming"
)

type (
	CmdSecGenCert struct {
		OptsGlobal
	}
)

func (t *CmdSecGenCert) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("gencert"),
		//objectaction.WithRemoteOptions(map[string]interface{}{}),
		objectaction.WithLocalRun(func(ctx context.Context, p naming.Path) (interface{}, error) {
			o, err := object.New(p)
			if err != nil {
				return nil, err
			}
			store, ok := o.(object.SecureKeystore)
			if !ok {
				return nil, fmt.Errorf("%s is not a secure keystore", o)
			}
			return nil, store.GenCert()
		}),
	).Do()
}
