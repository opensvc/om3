package entrypoints

import (
	"encoding/json"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/output"
)

// DaemonStats fetches and renders the statistic metrics from an opensvc
// agent api.
type DaemonStats struct {
	Color  string
	Format string
	Server string
}

// Do prints the formatted object selection
func (t DaemonStats) Do() error {
	var (
		err  error
		b    []byte
		data cluster.Stats
		c    *client.T
	)
	c, err = client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	handle := c.NewGetDaemonStats()
	b, err = handle.Do()
	if err != nil {
		return err
	}
	data, err = parseDaemonStats(b)
	if err != nil {
		return err
	}
	renderer := output.Renderer{
		Format:   t.Format,
		Color:    t.Color,
		Data:     data,
		Colorize: config.Node.Colorize,
	}
	renderer.Print()
	return nil
}

func parseDaemonStats(b []byte) (cluster.Stats, error) {
	type (
		nodeData struct {
			Status int               `json:"status"`
			Data   cluster.NodeStats `json:"data"`
		}
		responseType struct {
			Status int                 `json:"status"`
			Nodes  map[string]nodeData `json:"nodes"`
		}
	)
	var t responseType
	ds := make(cluster.Stats)
	err := json.Unmarshal(b, &t)
	if err != nil {
		return ds, err
	}
	for k, v := range t.Nodes {
		ds[k] = v.Data
	}
	return ds, nil
}
