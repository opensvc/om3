package commoncmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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

	// clusterTombstoneView is a row of the table: a tombstone a node wrote.
	clusterTombstoneView struct {
		Path            string    `json:"path"`
		Kind            string    `json:"kind"`
		Node            string    `json:"node"`
		Version         time.Time `json:"version"`
		DeletedAt       time.Time `json:"deleted_at"`
		Holders         string    `json:"holders"`
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
		Long: `A tombstone records a delete or a purge: the version of the config a node
deleted, its modification time. A config of that version or older, as one a
node away during the delete still holds, is the deleted one: no node fetches
it, nor serves it to a peer. A node up that missed the delete completes it.
A node returning with it keeps its instance stopped, the issue shown, until
the operator purges it there, or keeps it by an edit or a touch of its
config, a newer version.`,
	}
}

// NewCmdClusterTombstoneList returns the command listing the tombstones.
func NewCmdClusterTombstoneList() *cobra.Command {
	var options CmdClusterTombstoneList
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list the tombstones of the cluster, a row per node writing one",
		Long: `A node writes a tombstone as it deletes its instance, when a node may
still hold the version deleted, and removes it once every node is up, none
publishes that version, and each node that may have held it was seen
deleting its instance, or a day later. The holders are the nodes
publishing it.

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
		DefaultOutput: "tab=PATH:path,KIND:kind,NODE:node,VERSION:version,DELETED_AT:deleted_at,HOLDERS:holders,ORCHESTRATION_ID:orchestration_id",
		Output:        t.Output,
		DefaultSort:   "path",
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// clusterTombstoneViews are the tombstones as the table shows them.
func clusterTombstoneViews(items []api.Tombstone) []clusterTombstoneView {
	views := make([]clusterTombstoneView, 0, len(items))
	for _, item := range items {
		views = append(views, clusterTombstoneView{
			Path:            item.Path,
			Kind:            string(item.Kind),
			Node:            item.Node,
			Version:         item.Version,
			DeletedAt:       item.DeletedAt,
			Holders:         strings.Join(item.Holders, " "),
			OrchestrationID: item.OrchestrationID,
		})
	}
	return views
}
