package monitor

import (
	"fmt"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/output"
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

func doWatch(api client.API, data cluster.Status, color string, format string) {
	opts := client.NewEventsCmdConfig()
	events, _ := api.Events(*opts)
	defer close(events)
	doOneshot(data, color, format)
	for event := range events {
		fmt.Println("xx", event)
		doOneshot(data, color, format)
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
