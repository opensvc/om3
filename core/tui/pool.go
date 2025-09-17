package tui

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/sizeconv"
	"github.com/rivo/tview"
)

func (t *App) getPoolList() map[string]pool.Status {
	m := make(map[string]pool.Status)
	for _, nodeData := range t.Current.Cluster.Node {
		for poolName, poolData := range nodeData.Pool {
			item, ok := m[poolName]
			if !ok {
				item = poolData
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

func (t *App) updatePoolList() {
	if t.skipIfPoolNotUpdated() {
		return
	}
	title := "Storage Pools"
	titles := []string{"NAME", "TYPE", "CAPABILITIES", "HEAD", "VOLUME_COUNT", "BIN_SIZE", "BIN_USED", "BIN_FREE"}
	var elementsList [][]string

	m := t.getPoolList()

	for poolName, poolData := range m {
		elements := []string{
			poolName,
			poolData.Type,
			strings.Join(poolData.Capabilities, ","),
			poolData.Head,
			strconv.FormatInt(int64(poolData.VolumeCount), 10),
			sizeconv.BSizeCompact(float64(poolData.Size)),
			sizeconv.BSizeCompact(float64(poolData.Used)),
			sizeconv.BSizeCompact(float64(poolData.Free)),
		}
		elementsList = append(elementsList, elements)
	}

	t.createTableE(title, titles, elementsList, func(event *tcell.EventKey, v *tview.Table) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			row, _ := v.GetSelection()
			if row == 0 {
				break
			}
			poolName := v.GetCell(row, 0).Text
			t.selectedElement = poolName
			t.nav(viewPoolVolume)
		}

		return event
	})
}

func (t *App) updatePoolVolume(name string) {
	title := fmt.Sprintf("Storage Pool %s Volumes", name)
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

	t.createTable(title, titles, elementsList)
}
