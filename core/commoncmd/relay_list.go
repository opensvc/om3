package commoncmd

import (
	"strings"
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

// RenderRelayList prints the clients of the relays. A table shows the views,
// with the AGE rendered from the update time. A machine format prints the
// api answer as it is, a list of its kind, as the api documents it: a
// script reads the same whether it asks the api or the command.
func RenderRelayList(data api.RelayStatusList, out, sort, color string) error {
	v := ListingData(out, data, func() any { return ToRelayViews(data.Items, time.Now()) })
	return output.Renderer{
		DefaultOutput: relayListColumns,
		Output:        out,
		DefaultSort:   relayListSort,
		Sort:          sort,
		Color:         color,
		Data:          v,
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// IsTabularOutput says the output format out, the default one when empty,
// renders columns for a reader, rather than data for a program. A listing
// renders its views, computed for the reader, in a tabular format, and the
// api answer as it is in the others: a script reads the same document from
// the command and from the api.
// ListingData returns what a listing renders in the output format out: the
// views, computed for a reader, in a tabular format, and the api answer as
// it is in the others.
func ListingData(out string, answer any, views func() any) any {
	if IsTabularOutput(out) {
		return views()
	}
	return answer
}

func IsTabularOutput(out string) bool {
	format, _, _ := strings.Cut(out, "=")
	switch output.New(format) {
	case output.Tab, output.Table, output.CSV, output.Human:
		// The default output, and an unknown name, read as human,
		// render the default columns.
		return true
	}
	return false
}
