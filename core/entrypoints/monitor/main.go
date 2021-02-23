package monitor

import (
	"fmt"
	"time"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/patch"
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
		handleEvent(data, event)
		doOneshot(*data, color, format)
	}
}

func handleEvent(data *cluster.Status, e event.Event) {
	switch e.Kind {
	case "event":
		return
	case "patch":
		patchset := patch.NewSet(e.Data.([]interface{}))
		patchData(data, patchset)
	default:
		// unexpected: avoid fast looping
		time.Sleep(100 * time.Millisecond)
		return
	}
}

func patchData(data *cluster.Status, patchset patch.SetType) {
	fmt.Println("patching", "with", patchset)
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
