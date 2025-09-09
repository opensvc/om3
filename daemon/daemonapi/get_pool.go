package daemonapi

import (
	"net/http"
	"sort"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/daemon/api"
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
			return JSONProblemf(ctx, http.StatusInternalServerError, "Internal Server Error", "expand node selection %s: %w", selector, err)
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
		item := api.Pool{
			Capabilities: append([]string{}, stat.Capabilities...),
			Free:         stat.Free,
			Head:         stat.Head,
			Name:         e.Name,
			Node:         e.Node,
			Size:         stat.Size,
			Type:         stat.Type,
			Used:         stat.Used,
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
		if !ok {
			item = api.Pool{
				Capabilities: append([]string{}, stat.Capabilities...),
				Free:         stat.Free,
				Head:         stat.Head,
				Name:         e.Name,
				Size:         stat.Size,
				Type:         stat.Type,
				Used:         stat.Used,
				VolumeCount:  len(getPoolVolumes(&e.Name)),
			}
		} else if !stat.Shared {
			item.Free += stat.Free
			item.Size += stat.Size
			item.Used += stat.Used
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
