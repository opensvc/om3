package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectSetProvisioned is the cobra flag set of the set provisioned command.
	CmdObjectSetProvisioned struct {
		OptsGlobal
		object.OptsSetProvisioned
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectSetProvisioned) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectSetProvisioned) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "provisioned",
		Short:   "Set the resources as provisioned.",
		Long:    "This action does not provision the resources (fs are not formatted, disk not allocated, ...). This is just a resources provisioned flag create. Necessary to allow the unprovision action, which is bypassed if the provisioned flag is not set.",
		Aliases: []string{"provision", "prov"},
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectSetProvisioned) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("set provisioned"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"rid": t.RID,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewActorFromPath(p)
			if err != nil {
				return nil, err
			}
			return nil, o.SetProvisioned(t.OptsSetProvisioned)
		}),
	).Do()
}
