package monitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/inancgumus/screen"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/delta"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/output"
)

// Do renders the cluster status
func Do(selector string, watch bool, color string, format string) {
	api := client.New()

	if watch {
		if err := doWatch(api, selector, color, format); err != nil {
			fmt.Println(err)
		}
		return
	}
	opts := client.NewDaemonStatusOptions()
	opts.ObjectSelector = selector
	data, err := api.DaemonStatus(*opts)
	if err != nil {
		return
	}
	doOneshot(data, color, format, false)
}

func doWatch(api client.API, selector string, color string, format string) error {
	var (
		data cluster.Status
		ok   bool
	)
	opts := client.NewEventsOptions()
	opts.Full = true
	opts.ObjectSelector = selector
	events, _ := api.EventsRaw(*opts)
	first, ok := <-events
	if !ok {
		return errors.New("event channel unexpectedly closed")
	}
	b, ok := first.([]byte)
	if !ok {
		return errors.New("first event channel message is not a byte array")
	}
	evt, err := event.DecodeFromJSON(b)
	if err != nil {
		return err
	}
	b = *evt.Data
	json.Unmarshal(*evt.Data, &data)
	doOneshot(data, color, format, true)
	for m := range events {
		e, ok := m.([]byte)
		if !ok {
			continue
		}
		evt, err := event.DecodeFromJSON(e)
		if err != nil {
			fmt.Println(err, string(e))
			continue
		}
		err = handleEvent(&b, evt)
		if err != nil {
			return err
		}
		json.Unmarshal(b, &data)
		doOneshot(data, color, format, true)
	}
	return nil
}

func handleEvent(b *[]byte, e event.Event) error {
	var err error
	switch e.Kind {
	case "event":
		return nil
	case "patch", "full":
		patch := delta.NewPatch(*e.Data)
		*b, err = patch.Apply(*b)
		if err != nil {
			return err
		}
	default:
		// unexpected: avoid fast looping
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func doOneshot(data cluster.Status, color string, format string, clear bool) {
	human := func() string {
		return cluster.Render(
			cluster.Data{Current: data},
			cluster.Options{},
		)
	}

	s := output.Switch(format, color, data, human)
	if clear {
		screen.Clear()
		screen.MoveTopLeft()
	}
	fmt.Print(s)
}
