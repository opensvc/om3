package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectCertificateSigningRequest struct {
		OptsGlobal
	}
)

func (t *CmdObjectCertificateSigningRequest) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			o, err := object.New(p)
			if err != nil {
				return nil, err
			}
			store, ok := o.(object.KeyStore)
			if !ok {
				return nil, fmt.Errorf("%s is not a keystore", p)
			}
			if b, err := store.GenCertificateSigningRequest(); err != nil {
				return nil, fmt.Errorf("%s can't create certificate signing request: %w", p, err)
			} else {
				fmt.Println(string(b))
				return nil, nil
			}
		}),
	).Do()
}
