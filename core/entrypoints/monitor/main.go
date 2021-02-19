package monitor

import (
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/render/cluster"
)

// Do renders the cluster status
func Do(watch bool, color string, formatStr string) {
	api := client.New()
	data, err := api.DaemonStatus()
	if err != nil {
		return
	}

	human := func() {
		cluster.Render(
			cluster.Data{Current: data},
			cluster.Options{},
		)
	}

	output.Switch(formatStr, color, data, human)
}
