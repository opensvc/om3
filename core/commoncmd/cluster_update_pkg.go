package commoncmd

import (
	"context"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/actionrouter"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/nodeselector"
)

type CmdClusterUpdatePkg struct {
	OptsAsync
	Color  string
	Output string
	Sort   string
}

// NewCmdClusterUpdate is the parent of the cluster update commands. It is
// the orchestrated one, its commands sitting under it.
func NewCmdClusterUpdate() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDOrchestrated,
		Use:     "update",
		Short:   "update the opensvc packages of the cluster nodes",
	}
}

func NewCmdClusterUpdatePkg() *cobra.Command {
	var options CmdClusterUpdatePkg
	cmd := &cobra.Command{
		Use:   "pkg",
		Args:  cobra.NoArgs,
		Short: "upgrade the opensvc packages of all the nodes, one node at a time",
		Long: `Upgrade the opensvc packages of all the cluster nodes, one node at a time.

The nodes are asked by an orchestration, as a freeze of the cluster is, which
--wait waits for the end of and "om cluster abort" stops. Each node runs
"om node update pkg" in turn, in the order of the cluster nodes, once every
node of the cluster is live and idle: the upgrade of opensvc-server restarts
the daemon, and the next node waits for the one before it to be back. The
upgrade is given up when it fails on a node, and refused while a node of the
cluster is not live.

A node upgrading is in the "upgrade" state, which the peers treat as a
maintenance: its data is kept while its daemon restarts, and the services it
runs go on running.

The wait of --wait goes through the restart of the daemon the command talks
to, and holds until every node is back idle: an hour at most by default, a
package manager run, a daemon restart and a rejoin per node taking longer
than the few minutes other orchestrations are given. The logs --follow
streams stop at that restart.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)
	FlagSort(flags, &options.Sort)
	FlagsAsync(flags, &options.OptsAsync)
	// A cluster takes longer to upgrade than the default wait of an
	// orchestration: a package manager run, a daemon restart and a rejoin
	// per node.
	if f := flags.Lookup("time"); f != nil {
		_ = f.Value.Set("1h")
		f.DefValue = "1h"
	}
	return cmd
}

func (t *CmdClusterUpdatePkg) Run() error {
	// The nodes to wait for are read before the upgrade is asked: the daemon
	// asked may be the first to restart.
	var nodes []string
	if t.Wait || t.Follow {
		c, err := client.New()
		if err != nil {
			return err
		}
		if nodes, err = nodeselector.New("*", nodeselector.WithClient(c)).Expand(); err != nil {
			return err
		}
	}
	return nodeaction.New(
		nodeaction.WithAsyncTarget(node.MonitorGlobalExpectUpgraded.String()),
		nodeaction.WithAsyncTime(t.Time),
		nodeaction.WithAsyncWait(t.Wait),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithAsyncFollow(t.Follow),
		nodeaction.WithAsyncWaitFunc(func(ctx context.Context, c *client.T, id uuid.UUID, _ string) error {
			return actionrouter.WaitOrchestrationThroughRestarts(ctx, c, id, nodes)
		}),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocal(false),
	).Do()
}
