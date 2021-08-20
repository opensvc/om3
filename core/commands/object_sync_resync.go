package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectSyncResync is the cobra flag set of the sync resync command.
	CmdObjectSyncResync struct {
		object.OptsSyncResync
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectSyncResync) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectSyncResync) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "resync",
		Short: "restore optimal synchronization",
		Long:  "Only a subset of drivers support this interface. For example, the disk.md driver re-adds removed disks.",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectSyncResync) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.OptsGlobal.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.OptsGlobal.Local),
		objectaction.WithFormat(t.OptsGlobal.Format),
		objectaction.WithColor(t.OptsGlobal.Color),
		objectaction.WithRemoteNodes(t.OptsGlobal.NodeSelector),
		objectaction.WithRemoteAction("sync resync"),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			return nil, object.NewActorFromPath(p).SyncResync(t.OptsSyncResync)
		}),
	).Do()
}
