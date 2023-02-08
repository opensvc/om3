package commands

import (
	"encoding/json"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
)

type (
	CmdDaemonStats struct {
		OptsGlobal
	}
)

func (t *CmdDaemonStats) Run() error {
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
		Colorize: rawconfig.Colorize,
	}
	renderer.Print()
	return nil
}

func parseDaemonStats(b []byte) (cluster.Stats, error) {
	type (
		nodeData struct {
			Status int                     `json:"status"`
			Data   cluster.NodeStatsBundle `json:"data"`
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
