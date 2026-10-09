package daemonapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/locktable"
)

// lockTableRebuilding serializes the rebuilds of the lock table: two run at
// once would each replace the table, the later one without what the earlier
// granted in between.
var lockTableRebuilding sync.Mutex

// rebuildLockTable rebuilds the lock table this node grants from, from what
// the clients of every node hold, unless it is rebuilt already.
//
// The table is in memory, and is this node's to keep only while it speaks for
// the cluster. A node taking the speaking over has the table it had when it
// last spoke, or none, while the clients of the cluster hold the locks the
// node speaking before granted. Every node records the locks its own clients
// hold, which is what the table is rebuilt from.
//
// A node that does not answer leaves the table not rebuilt, and no lock is
// granted: the locks its clients hold are the ones a grant would break.
func (a *DaemonAPI) rebuildLockTable(ctx echo.Context) error {
	lockTableRebuilding.Lock()
	defer lockTableRebuilding.Unlock()
	table := locktable.SpeakerTable
	if table.IsRebuilt() {
		return nil
	}
	generation := table.Generation()
	locks := locktable.LocalHeld.List()
	asked := 0
	for _, nodename := range clusternode.Get() {
		if nodename == a.localhost {
			continue
		}
		if node.StatusData.GetByNode(nodename) == nil {
			// A node this one has no data of is not alive, and neither are
			// the clients it had.
			continue
		}
		l, err := nodeLocks(ctx.Request().Context(), nodename)
		if err != nil {
			return fmt.Errorf("ask %s the cluster locks its clients hold: %w", nodename, err)
		}
		locks = append(locks, l...)
		asked++
	}
	conflicts, ok := table.Rebuild(generation, locks)
	if !ok {
		return locktable.ErrNotRebuilt
	}
	log := LogHandler(ctx, "rebuildLockTable")
	for _, lock := range conflicts {
		log.Warnf("cluster lock %s granted twice: %s on %s holds it until %s too", lock.Name, lock.Holder, lock.Node, lock.ExpiresAt.Format(time.RFC3339))
	}
	log.Infof("cluster locks: table rebuilt from %d peers, %d lock(s) held", asked, len(table.List()))
	return nil
}

// nodeLocks returns the cluster locks the clients of a peer hold.
func nodeLocks(ctx context.Context, nodename string) ([]locktable.Lock, error) {
	// The node asks, not whoever asked it: a lock is no user's to see.
	c, err := newPeerClient(nodename)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := c.GetNodeLocksWithResponse(ctx, nodename)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode())
	}
	l := make([]locktable.Lock, len(resp.JSON200.Items))
	for i, item := range resp.JSON200.Items {
		l[i] = lockFromAPI(item)
	}
	return l, nil
}

func lockFromAPI(item api.ClusterLock) locktable.Lock {
	lock := locktable.Lock{
		Name:       item.Name,
		ID:         item.ID,
		Node:       item.Node,
		AcquiredAt: item.AcquiredAt,
		ExpiresAt:  item.ExpiresAt,
	}
	if item.Holder != nil {
		lock.Holder = *item.Holder
	}
	return lock
}

func lockToAPI(lock locktable.Lock) api.ClusterLock {
	item := api.ClusterLock{
		Name:       lock.Name,
		ID:         lock.ID,
		Node:       lock.Node,
		AcquiredAt: lock.AcquiredAt,
		ExpiresAt:  lock.ExpiresAt,
	}
	if lock.Holder != "" {
		holder := lock.Holder
		item.Holder = &holder
	}
	return item
}

func lockListToAPI(locks []locktable.Lock) api.ClusterLockList {
	items := make([]api.ClusterLock, len(locks))
	for i, lock := range locks {
		items[i] = lockToAPI(lock)
	}
	return api.ClusterLockList{Kind: "ClusterLockList", Items: items}
}
