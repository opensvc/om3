package daemonapi

import (
	"net/http"
	"sort"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetPools(ctx echo.Context, params api.GetPoolsParams) error {
	var (
		items   api.PoolItems
		nodeMap nodeselector.ResultMap
		err     error
	)
	if v, err := assertRoot(ctx); !v {
		return err
	}

	if params.Node != nil {
		selector := *params.Node
		selector = a.parseNodename(selector)
		selection := nodeselector.New(selector)
		nodeMap, err = selection.ExpandMap()
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Internal Server Error", "expand node selection %s: %s", selector, err)
		}
	}

	if nodeMap == nil {
		items = a.getClusterPools(ctx, params.Name)
	} else {
		items = a.getNodePools(ctx, params.Name, nodeMap)
	}

	sort.Slice(items, func(i, j int) bool {
		// First, compare by Node.
		if items[i].Node != items[j].Node {
			return items[i].Node < items[j].Node
		}
		// If Nodes are the same, compare by Name.
		return items[i].Name < items[j].Name
	})

	return ctx.JSON(http.StatusOK, api.PoolList{Kind: "PoolList", Items: items})
}

func (a *DaemonAPI) getNodePools(ctx echo.Context, name *string, nodeMap nodeselector.ResultMap) api.PoolItems {
	var items api.PoolItems
	for _, e := range pool.StatusData.GetAll() {
		if name != nil && *name != e.Name {
			continue
		}
		if !nodeMap.Has(e.Node) {
			continue
		}
		stat := *e.Value
		capabilities := make([]string, len(stat.Capabilities))
		for i, c := range stat.Capabilities {
			capabilities[i] = string(c)
		}
		item := api.Pool{
			Capabilities: capabilities,
			Free:         stat.Free,
			Head:         stat.Head,
			LogicalFree:  stat.LogicalFree,
			LogicalSize:  stat.LogicalSize,
			LogicalUsed:  stat.LogicalUsed,
			Name:         e.Name,
			Node:         e.Node,
			Shared:       stat.Shared,
			Size:         stat.Size,
			Type:         stat.Type,
			Used:         stat.Used,
			UpdatedAt:    stat.UpdatedAt,
			VolumeCount:  len(getPoolVolumes(&e.Name)),
		}
		if len(stat.Errors) > 0 {
			l := append([]string{}, stat.Errors...)
			item.Errors = &l
		}
		items = append(items, item)
	}
	return items
}

func (a *DaemonAPI) getClusterPools(ctx echo.Context, name *string) api.PoolItems {
	var items api.PoolItems
	m := make(map[string]api.Pool)
	for _, e := range pool.StatusData.GetAll() {
		if name != nil && *name != e.Name {
			continue
		}
		item, ok := m[e.Name]
		stat := *e.Value
		capabilities := make([]string, len(stat.Capabilities))
		for i, c := range stat.Capabilities {
			capabilities[i] = string(c)
		}
		if !ok {
			item = api.Pool{
				Capabilities: capabilities,
				Free:         stat.Free,
				Head:         stat.Head,
				LogicalFree:  stat.LogicalFree,
				LogicalSize:  stat.LogicalSize,
				LogicalUsed:  stat.LogicalUsed,
				Name:         e.Name,
				Shared:       stat.Shared,
				Size:         stat.Size,
				Type:         stat.Type,
				Used:         stat.Used,
				UpdatedAt:    stat.UpdatedAt,
				VolumeCount:  len(getPoolVolumes(&e.Name)),
			}
		} else if !stat.Shared {
			item.Free += stat.Free
			item.Size += stat.Size
			item.Used += stat.Used
			leastRoom(&item, stat)
			if item.UpdatedAt.Before(stat.UpdatedAt) {
				item.UpdatedAt = stat.UpdatedAt
			}
		}
		if len(stat.Errors) > 0 {
			l := append([]string{}, stat.Errors...)
			item.Errors = &l
		}
		m[e.Name] = item
	}
	for _, item := range m {
		items = append(items, item)
	}
	return items
}

// leastRoom keeps the logical figures of the node with the least room left.
//
// The storage of a pool that is not shared adds up across the nodes, and what
// it can hand out does not: a volume held on every node it spans needs its
// size on each of them, so what the pool can still hand out is what the node
// with the least room can take. Reporting the sum there would promise several
// times over what is really free.
//
// The three figures are kept together, from the one node, so that they still
// add up: a size, what has been handed out of it, and what is left.
//
// A node with nothing to say is not the node with the least room. Reading the
// usage of a pool can fail, and a pool can have no size of its own, and
// neither is a pool that can hand out nothing.
func leastRoom(item *api.Pool, stat pool.Status) {
	if stat.LogicalSize <= 0 {
		return
	}
	if item.LogicalSize > 0 && item.LogicalFree <= stat.LogicalFree {
		return
	}
	item.LogicalFree = stat.LogicalFree
	item.LogicalSize = stat.LogicalSize
	item.LogicalUsed = stat.LogicalUsed
}
