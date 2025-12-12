package tui

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

func (t *App) getPoolList() map[string]PoolData {
	m := make(map[string]PoolData)
	for nodeName, nodeData := range t.Current.Cluster.Node {
		for poolName, poolData := range nodeData.Pool {
			item, ok := m[poolName]
			if !ok {
				item = PoolData{
					Status: poolData,
					Node:   nodeName,
				}
			} else if !poolData.Shared {
				item.Free += poolData.Free
				item.Used += poolData.Used
				item.Size += poolData.Size
				if item.UpdatedAt.Before(poolData.UpdatedAt) {
					item.UpdatedAt = poolData.UpdatedAt
				}
			}
			m[poolName] = item
		}
	}
	return m
}

func (t *App) getPool(name string) []PoolData {
	var items []PoolData
	for nodeName, nodeData := range t.Current.Cluster.Node {
		for poolName, poolData := range nodeData.Pool {
			if name != "" && name != poolName {
				continue
			}
			items = append(items, PoolData{
				Status: poolData,
				Node:   nodeName,
			})
		}
	}
	return items
}

func (t *App) skipIfPoolNotUpdated() bool {
	m := t.getPoolList()
	var lastUpdate time.Time
	for _, poolData := range m {
		if poolData.UpdatedAt.After(lastUpdate) {
			lastUpdate = poolData.UpdatedAt
		}
	}
	if lastUpdate.IsZero() {
		return true
	}
	if lastUpdate.After(t.lastUpdatedAt) {
		t.lastUpdatedAt = lastUpdate
		return false
	}
	return true
}

func (t *App) updatePoolList(forceUpdate bool) {
	if !forceUpdate && t.skipIfPoolNotUpdated() {
		return
	}
	title := "pools"
	titles := []string{"NAME", "TYPE", "CAPABILITIES", "HEAD", "VOLUME_COUNT", "BIN_SIZE", "BIN_USED", "BIN_FREE"}
	if t.selectedElement != "" {
		title = fmt.Sprintf("%s pool", t.selectedElement)
		titles[0] = "NODE"
	}
	var elementsList [][]string

	buildElements := func(poolName string, poolData PoolData) []string {
		return []string{
			poolName,
			poolData.Type,
			strings.Join(poolData.Capabilities, ","),
			poolData.Head,
			strconv.FormatInt(int64(poolData.VolumeCount), 10),
			sizeconv.BSizeCompact(float64(poolData.Size)),
			sizeconv.BSizeCompact(float64(poolData.Used)),
			sizeconv.BSizeCompact(float64(poolData.Free)),
		}
	}

	if t.selectedElement == "" {
		m := t.getPoolList()

		poolNames := make([]string, 0, len(m))
		for poolName, _ := range m {
			poolNames = append(poolNames, poolName)
		}

		sort.Strings(poolNames)

		for _, poolName := range poolNames {
			elementsList = append(elementsList, buildElements(poolName, m[poolName]))
		}
	} else {
		pools := t.getPool(t.selectedElement)

		sort.Slice(pools, func(i, j int) bool {
			return pools[i].Size > pools[j].Size
		})

		for _, poolData := range pools {
			elements := buildElements(t.selectedElement, poolData)
			elements[0] = poolData.Node
			elementsList = append(elementsList, elements)
		}
	}

	selectableColumns := []int{0}
	if t.selectedElement == "" {
		selectableColumns = append(selectableColumns, 4)
	}

	t.createTable(CreateTableOptions{
		title:             title,
		titles:            titles,
		elementsList:      elementsList,
		selectableColumns: selectableColumns,
		capture: func(event *tcell.EventKey, v *tview.Table) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyEnter:
				row, col := v.GetSelection()
				if row == 0 {
					break
				}
				poolName := v.GetCell(row, 0).Text
				if col == 0 && t.selectedElement != "" {
					t.previousSelectedElement = t.selectedElement
				}
				t.selectedElement = poolName
				if col == 0 {
					t.nav(viewPool)
					t.position = Position{row: 0, col: 0}
					t.updatePoolList(forceUpdate)
				} else if col == 4 {
					t.nav(viewPoolVolume)
				}
			}
			return event
		},
	})
}

func (t *App) updatePoolVolume(name string) {
	title := fmt.Sprintf("%s volumes", name)
	titles := []string{"POOL", "PATH", "SIZE", "CHILDREN", "IS_ORPHAN"}
	var elementsList [][]string

	c, err := client.New()
	if err != nil {
		t.errorf("failed to create client: %s", err)
		return
	}

	params := api.GetPoolVolumesParams{}
	params.Name = &name
	resp, err := c.GetPoolVolumesWithResponse(context.Background(), &params)
	if err != nil {
		t.errorf("failed to get pool volumes: %s", err)
		return
	}

	if resp.StatusCode() != http.StatusOK {
		switch resp.StatusCode() {
		case 401:
			t.errorf("%s", resp.JSON401)
		case 403:
			t.errorf("%s", resp.JSON403)
		case 500:
			t.errorf("%s", resp.JSON500)
		default:
			t.errorf("unexpected status code: %d", resp.StatusCode())
		}
	}

	data := resp.JSON200
	for _, volume := range data.Items {
		elements := []string{
			volume.Pool,
			volume.Path,
			sizeconv.BSizeCompact(float64(volume.Size)),
			strings.Join(volume.Children, ","),
			strconv.FormatBool(volume.IsOrphan),
		}
		elementsList = append(elementsList, elements)
	}

	t.createTable(CreateTableOptions{
		title:             title,
		titles:            titles,
		elementsList:      elementsList,
		selectableColumns: []int{0},
	})
}
