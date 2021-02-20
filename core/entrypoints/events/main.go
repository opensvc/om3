package events

import (
	"fmt"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/output"
)

// Do renders the cluster status
func Do(color string, format string) {
	api := client.New()
	opts := client.NewEventsCmdConfig()
	events, _ := api.Events(*opts)
	defer close(events)
	for event := range events {
		doOne(event, color, format)
	}
}

func doOne(event client.Event, color string, format string) {
	human := func() {
		fmt.Println(event)
	}
	output.Switch(format, color, event, human)
}
