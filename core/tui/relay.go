package tui

import (
	"context"
	"net/http"
	"strconv"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

func (t *App) updateRelayStatus() {
	title := "relay"
	titles := []string{"RELAY", "USERNAME", "CLUSTER_ID", "CLUSTER_NAME", "NODENAME", "NODE_ADDR", "UPDATED_AT", "MSG_LEN"}
	var elementsList [][]string

	cli, err := client.New()
	if err != nil {
		t.errorf("failed to create client: %s", err)
		return
	}

	params := api.GetRelayStatusParams{}
	resp, err := cli.GetRelayStatusWithResponse(context.Background(), &params)
	if err != nil {
		t.errorf("failed to get relays status: %s", err)
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

	data := resp.JSON200
	for _, relay := range data.Items {
		elements := []string{
			relay.Relay,
			relay.Username,
			relay.ClusterID,
			relay.ClusterName,
			relay.Nodename,
			relay.NodeAddr,
			relay.UpdatedAt.Format("2006-01-02 15:04:05"),
			strconv.Itoa(relay.MsgLen),
		}
		elementsList = append(elementsList, elements)
	}

	t.createTable(CreateTableOptions{
		title:        title,
		titles:       titles,
		elementsList: elementsList,
	})

}
