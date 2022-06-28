package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdKeystoreRemove is the cobra flag set of the remove command.
	CmdSecGenCert struct {
		object.OptsGenCert
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdSecGenCert) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsGenCert)
}

func (t *CmdSecGenCert) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "gencert",
		Short: "create or replace a x509 certificate stored as a keyset",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdSecGenCert) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("gencert"),
		//objectaction.WithRemoteOptions(map[string]interface{}{}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewFromPath(p)
			if err != nil {
				return nil, err
			}
			store, ok := o.(object.SecureKeystorer)
			if !ok {
				return nil, fmt.Errorf("%s is not a secure keystore", o)
			}
			return nil, store.GenCert(t.OptsGenCert)
		}),
	).Do()
}
