package monitor

import (
	"fmt"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/render/cluster"
	"opensvc.com/opensvc/core/types"
)

// Do renders the cluster status
func Do(watch bool, color string, format string) {
	api := client.New()
	data, err := api.DaemonStatus()
	if err != nil {
		return
	}

	if watch {
		doWatch(api, data, color, format)
	}
	doOneshot(data, color, format)
}

func doWatch(api client.API, data types.DaemonStatus, color string, format string) {
	opts := client.NewEventsCmdConfig()
	events, _ := api.Events(*opts)
	defer close(events)
	doOneshot(data, color, format)
	for event := range events {
		fmt.Println("xx", event)
		doOneshot(data, color, format)
	}
}

func doOneshot(data types.DaemonStatus, color string, format string) {
	human := func() {
		cluster.Render(
			cluster.Data{Current: data},
			cluster.Options{},
		)
	}

	output.Switch(format, color, data, human)
}
