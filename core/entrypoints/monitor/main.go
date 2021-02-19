package monitor

import (
	"encoding/json"
	"fmt"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/render"
	"opensvc.com/opensvc/core/render/cluster"
)

// Do renders the cluster status
func Do(watch bool, color string, formatStr string) {
	format := output.New(formatStr)
	render.SetColor(color)
	api := client.New()
	data, err := api.DaemonStatus()
	if err != nil {
		return
	}

	switch format {
	case output.Flat:
		var b []byte
		b, err = json.MarshalIndent(data, "", "    ")
		output.PrintFlat(b)
	case output.JSON:
		var b []byte
		b, err = json.MarshalIndent(data, "", "    ")
		fmt.Println(string(b))
	default:
		cluster.Render(
			cluster.Data{Current: data},
			cluster.Options{},
		)
	}

}
