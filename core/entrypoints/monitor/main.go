package monitor

import (
	"fmt"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/output"
)

// Do renders the cluster status
func Do(selector string, watch bool, color string, format string) {
	api := client.New()
	opts := client.NewDaemonStatusOptions()
	opts.ObjectSelector = selector
	data, err := api.DaemonStatus(*opts)
	if err != nil {
		return
	}

	if watch {
		doWatch(api, &data, selector, color, format)
	}
	doOneshot(data, color, format)
}

func doWatch(api client.API, data *cluster.Status, selector string, color string, format string) {
	opts := client.NewEventsOptions()
	opts.ObjectSelector = selector
	events, _ := api.Events(*opts)
	defer close(events)
	doOneshot(*data, color, format)
	for event := range events {
		fmt.Println("xx", event)
		doOneshot(*data, color, format)
	}
}

func doOneshot(data cluster.Status, color string, format string) {
	human := func() {
		cluster.Render(
			cluster.Data{Current: data},
			cluster.Options{},
		)
	}

	output.Switch(format, color, data, human)
}
