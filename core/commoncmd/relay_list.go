package commoncmd

import (
	"time"

	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/duration"
)

const (
	relayListColumns = "tab=RELAY:relay,USERNAME:username,CLUSTER_ID:cluster_id,CLUSTER_NAME:cluster_name,NODENAME:nodename,NODE_ADDR:node_addr,UPDATED_AT:updated_at,AGE:age,MSG_LEN:msg_len"

	// relayListSort lists the latest updates first: the live clients on
	// top, the stale ones below, as the slots of a client gone stay 24h.
	relayListSort = "-updated_at,cluster_id,nodename,node_addr"
)

// RelayView is what the table shows of a relay client: the api item and the
// age of its data.
type RelayView struct {
	api.RelayStatusItem

	// Age is rendered, not carried: how long ago the data was updated, by
	// the reader's clock.
	Age string `json:"age,omitempty"`
}

func ToRelayViews(items api.RelayStatusItems, now time.Time) []RelayView {
	l := make([]RelayView, 0, len(items))
	for _, i := range items {
		v := RelayView{RelayStatusItem: i}
		if !i.UpdatedAt.IsZero() {
			v.Age = duration.FmtShortDuration(now.Sub(i.UpdatedAt))
		}
		l = append(l, v)
	}
	return l
}

// RenderRelayList prints the clients of the relays.
func RenderRelayList(data api.RelayStatusList, out, sort, color string) error {
	return output.Renderer{
		DefaultOutput: relayListColumns,
		Output:        out,
		DefaultSort:   relayListSort,
		Sort:          sort,
		Color:         color,
		Data:          ToRelayViews(data.Items, time.Now()),
		Colorize:      rawconfig.Colorize,
	}.Print()
}
