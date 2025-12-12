package tui

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

func (t *App) updateNetworkList() {
	title := "Networks"
	titles := []string{"NAME", "TYPE", "NETWORK", "SIZE", "USED", "FREE"}
	var elementsList [][]string

	c, err := client.New()
	if err != nil {
		t.errorf("failed to create client: %s", err)
		return
	}

	params := api.GetNetworksParams{}
	resp, err := c.GetNetworksWithResponse(context.Background(), &params)
	if err != nil {
		t.errorf("failed to get networks: %s", err)
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
		return
	}

	items := resp.JSON200.Items

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	for _, network := range items {
		elements := []string{
			network.Name,
			network.Type,
			network.Network,
			sizeconv.BSizeCompact(float64(network.Size)),
			sizeconv.BSizeCompact(float64(network.Used)),
			sizeconv.BSizeCompact(float64(network.Free)),
		}
		elementsList = append(elementsList, elements)
	}

	t.createTable(CreateTableOptions{
		title:             title,
		titles:            titles,
		elementsList:      elementsList,
		selectableColumns: []int{0},
		capture: func(event *tcell.EventKey, v *tview.Table) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyEnter:
				row, _ := v.GetSelection()
				if row == 0 {
					break
				}
				networkName := v.GetCell(row, 0).Text
				t.selectedElement = networkName
				t.nav(viewNetworkIpList)
			}
			return event
		},
	})
}

func (t *App) updateNetworkIpList(name string) {
	title := fmt.Sprintf("Network %s IPs", name)
	titles := []string{"OBJECT", "NODE", "RID", "IP", "NET_NAME", "NET_TYPE"}
	var elementsList [][]string

	c, err := client.New()
	if err != nil {
		t.errorf("failed to create client: %s", err)
	}

	params := api.GetNetworkIPParams{}
	params.Name = &name
	resp, err := c.GetNetworkIPWithResponse(context.Background(), &params)
	if err != nil {
		t.errorf("failed to get network ips: %s", err)
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

	items := resp.JSON200.Items

	sort.Slice(items, func(i, j int) bool {
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		return items[i].IP < items[j].IP
	})

	for _, ip := range items {
		elements := []string{
			ip.Path,
			ip.Node,
			ip.RID,
			ip.IP,
			ip.Network.Name,
			ip.Network.Type,
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
