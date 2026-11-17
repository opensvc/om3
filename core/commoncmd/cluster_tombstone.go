package commoncmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	// CmdClusterTombstoneList lists the tombstones of the cluster.
	CmdClusterTombstoneList struct {
		OptsGlobal
	}

	// clusterTombstoneView is a row of the table: a holder of a tombstone.
	clusterTombstoneView struct {
		Path            string    `json:"path"`
		Kind            string    `json:"kind"`
		Holder          string    `json:"holder"`
		Mode            string    `json:"mode"`
		State           string    `json:"state"`
		ClearedBy       string    `json:"cleared_by,omitempty"`
		PurgedAt        time.Time `json:"purged_at"`
		OrchestrationID string    `json:"orchestration_id"`
	}
)

// NewCmdClusterTombstone returns the parent of the cluster tombstone
// commands.
func NewCmdClusterTombstone() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "tombstone",
		Short:   "cluster tombstone commands",
		Long: `A tombstone records a delete or a purge some nodes of the object did not run:
the nodes holding the instance still, each pending until its instance is gone
or kept there. The other nodes fetch no configuration of the object from a
pending holder, so the object does not come back from it.

A live holder was up but had not run the orchestration yet: its daemon
completes it. An away holder was down, in maintenance or unknown: its
instance is kept stopped, never deleted on its own, until the operator purges
it there, or keeps it by an edit or a touch of its configuration there.`,
	}
}

// NewCmdClusterTombstoneList returns the command listing the tombstones.
func NewCmdClusterTombstoneList() *cobra.Command {
	var options CmdClusterTombstoneList
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list the tombstones of the cluster, a row per holder",
		Long: `The tombstones listed are the ones the nodes publish, the copies merged. A
tombstone goes once all its holders are cleared and every node is up.

A user other than root sees the tombstones of the namespaces it is guest of.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagOutput(flags, &options.Output)
	FlagSort(flags, &options.Sort)
	FlagColor(flags, &options.Color)
	return cmd
}

func (t *CmdClusterTombstoneList) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := c.GetClusterTombstonesWithResponse(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return fmt.Errorf("list the tombstones: %s", resp.Status())
	}
	items := resp.JSON200.Items
	data := ListingData(t.Output, api.TombstoneList{Kind: api.TombstoneListKindTombstoneList, Items: items}, func() any {
		return clusterTombstoneViews(items)
	})
	return output.Renderer{
		DefaultOutput: "tab=PATH:path,KIND:kind,HOLDER:holder,MODE:mode,STATE:state,CLEARED_BY:cleared_by,PURGED_AT:purged_at,ORCHESTRATION_ID:orchestration_id",
		Output:        t.Output,
		DefaultSort:   "path",
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// clusterTombstoneViews are the tombstones as the table shows them, a row
// per holder.
func clusterTombstoneViews(items []api.Tombstone) []clusterTombstoneView {
	views := make([]clusterTombstoneView, 0)
	for _, item := range items {
		for _, h := range item.Holders {
			view := clusterTombstoneView{
				Path:            item.Path,
				Kind:            string(item.Kind),
				Holder:          h.Node,
				Mode:            string(h.Mode),
				State:           string(h.State),
				PurgedAt:        item.PurgedAt,
				OrchestrationID: item.OrchestrationID,
			}
			if h.ClearedBy != nil {
				view.ClearedBy = *h.ClearedBy
			}
			views = append(views, view)
		}
	}
	return views
}
