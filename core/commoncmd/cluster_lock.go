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
	// CmdClusterLockList lists the cluster locks held.
	CmdClusterLockList struct {
		OptsGlobal
	}

	// CmdClusterLockRelease takes a cluster lock back from its holder.
	CmdClusterLockRelease struct {
		Name string
		ID   string
	}

	// clusterLockView is what the table shows: a lock, and how long its
	// lease still runs.
	clusterLockView struct {
		Name       string    `json:"name"`
		ID         string    `json:"id"`
		Holder     string    `json:"holder,omitempty"`
		Node       string    `json:"node"`
		AcquiredAt time.Time `json:"acquired_at"`
		ExpiresAt  time.Time `json:"expires_at"`
		ExpiresIn  string    `json:"expires_in"`
	}
)

// NewCmdClusterLock returns the parent of the cluster lock commands.
func NewCmdClusterLock() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "lock",
		Short:   "cluster lock commands",
		Long: `A cluster lock serializes, across the nodes, what reads the state of the
cluster and changes it on the strength of that reading, as the drawing of an
address from a network every node draws from.

The node speaking for the cluster grants every lock, for a lease that frees
it should its holder die holding it.`,
	}
}

// NewCmdClusterLockList returns the command listing the cluster locks held.
func NewCmdClusterLockList() *cobra.Command {
	var options CmdClusterLockList
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list the cluster locks held",
		Long: `The locks listed are the ones the node speaking for the cluster granted, and
whose holder has neither released them nor seen their lease end.`,
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

// NewCmdClusterLockRelease returns the command taking a cluster lock back
// from its holder.
func NewCmdClusterLockRelease() *cobra.Command {
	var options CmdClusterLockRelease
	cmd := &cobra.Command{
		Use:   "release NAME",
		Short: "take a cluster lock back from its holder",
		Long: `Release a cluster lock its holder does not release, before its lease ends.

The holder is not told: it goes on as if it held the lock, which is what the
lock was for. This is for a holder known to be stuck, or gone without the
lease having ended yet.

Naming the id releases the lock only while it is still held under it, so a
lock released meanwhile and granted to another is left to that one.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Name = args[0]
			return options.Run()
		},
	}
	CmdWithArg(cmd, `NAME  The name of the lock, as the lock listing shows it.`)
	flags := cmd.Flags()
	flags.StringVar(&options.ID, "id", "", "release the lock only while it is held under this id")
	return cmd
}

func (t *CmdClusterLockList) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, err := clusterLocks(ctx, c)
	if err != nil {
		return err
	}
	now := time.Now()
	views := make([]clusterLockView, len(items))
	for i, item := range items {
		views[i] = clusterLockView{
			Name:       item.Name,
			ID:         item.ID,
			Node:       item.Node,
			AcquiredAt: item.AcquiredAt,
			ExpiresAt:  item.ExpiresAt,
			ExpiresIn:  item.ExpiresAt.Sub(now).Round(time.Second).String(),
		}
		if item.ExpiresIn != nil {
			// The time left as the node speaking counts it, rather than its
			// expiry read on the clock of this host.
			if d, err := time.ParseDuration(*item.ExpiresIn); err == nil {
				views[i].ExpiresIn = d.Round(time.Second).String()
			}
		}
		if item.Holder != nil {
			views[i].Holder = *item.Holder
		}
	}
	return output.Renderer{
		DefaultOutput: "tab=NAME:name,HOLDER:holder,NODE:node,ACQUIRED_AT:acquired_at,EXPIRES_IN:expires_in",
		Output:        t.Output,
		DefaultSort:   "name",
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          views,
		Colorize:      rawconfig.Colorize,
	}.Print()
}

func (t *CmdClusterLockRelease) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id := t.ID
	if id == "" {
		items, err := clusterLocks(ctx, c)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.Name == t.Name {
				id = item.ID
				break
			}
		}
		if id == "" {
			return fmt.Errorf("cluster lock %s is not held", t.Name)
		}
	}
	resp, err := c.DeleteClusterLockWithResponse(ctx, &api.DeleteClusterLockParams{Name: t.Name, Id: id})
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusNoContent:
		fmt.Printf("cluster lock %s released\n", t.Name)
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("cluster lock %s is not held under %s: it was released, or its lease ended", t.Name, id)
	default:
		return fmt.Errorf("release cluster lock %s: %s: %s", t.Name, resp.Status(), resp.Body)
	}
}

func clusterLocks(ctx context.Context, c *client.T) ([]api.ClusterLock, error) {
	resp, err := c.GetClusterLocksWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("list the cluster locks: %s: %s", resp.Status(), resp.Body)
	}
	return resp.JSON200.Items, nil
}
