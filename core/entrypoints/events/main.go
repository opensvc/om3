package events

import (
	"fmt"
	"os"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/output"
)

// Do renders the cluster status
func Do(color string, format string) {
	api, err := client.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	opts := client.NewEventsOptions()
	events, _ := api.Events(*opts)
	for m := range events {
		doOne(m.(event.Event), color, format)
	}
}

func doOne(e event.Event, color string, format string) {
	human := func() string {
		return event.Render(e)
	}
	if format == output.JSON.String() {
		format = output.JSONLine.String()
	}
	fmt.Print(output.Switch(format, color, e, human))
}
