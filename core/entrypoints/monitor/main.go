package monitor

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/delta"
	"opensvc.com/opensvc/core/event"
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
	} else {
		doOneshot(data, color, format)
	}
}

func doWatch(api client.API, data *cluster.Status, selector string, color string, format string) {
	opts := client.NewEventsOptions()
	opts.ObjectSelector = selector
	events, _ := api.Events(*opts)
	defer close(events)
	doOneshot(*data, color, format)
	b, _ := json.Marshal(data)
	for event := range events {
		handleEvent(&b, data, event)
		doOneshot(*data, color, format)
	}
}

func handleEvent(b *[]byte, data *cluster.Status, e event.Event) {
	var err error
	switch e.Kind {
	case "event":
		return
	case "patch":
		patch := delta.NewPatch(*e.Data)
		*b, err = patch.Apply(*b)
		if err != nil {
			panic(err)
		}
		json.Unmarshal(*b, data)
	default:
		// unexpected: avoid fast looping
		time.Sleep(100 * time.Millisecond)
		return
	}
}

func patchData(data interface{}, patch delta.Patch) {
	for _, o := range patch {
		applyOp(data, o)
	}
}

func applyOp(data interface{}, o delta.Operation) {
	path, _ := o.Path()
	for _, key := range path {
		switch key.(type) {
		case string:
			fmt.Println("patching", "string", key)
			data = data.(map[string]interface{})[key.(string)]
		case int:
			fmt.Println("patching", "int", key)
			data = data.([]interface{})[key.(int)]
		default:
			fmt.Println("xx", key, reflect.TypeOf(key), reflect.ValueOf(key))
		}
	}
	//data = o.value()
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
