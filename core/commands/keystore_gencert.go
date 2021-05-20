package commands

import (
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
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithRemoteAction("gencert"),
		//objectaction.WithRemoteOptions(map[string]interface{}{}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			return nil, object.NewFromPath(p).(object.SecureKeystorer).GenCert(t.OptsGenCert)
		}),
	).Do()
}
