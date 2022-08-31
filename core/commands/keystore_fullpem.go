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
	// CmdFullPEM is the cobra flag set of the remove command.
	CmdFullPEM struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdFullPEM) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdFullPEM) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "fullpem",
		Short: "dump the private_key and certificate_chain in PEM format",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdFullPEM) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("fullpem"),
		//objectaction.WithRemoteOptions(map[string]interface{}{}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.New(p)
			if err != nil {
				return nil, err
			}
			store, ok := o.(object.SecureKeystore)
			if !ok {
				return nil, fmt.Errorf("%s is not a secure keystore", o)
			}
			return store.FullPEM()
		}),
	).Do()
}
