package oxcmd

import (
	"encoding/json"
	"fmt"

	"github.com/opensvc/om3/core/clusterdump"
)

type (
	CmdDaemonStats struct {
		OptsGlobal
	}
)

func (t *CmdDaemonStats) Run() error {
	return fmt.Errorf("todo")
}

func parseDaemonStats(b []byte) (clusterdump.Stats, error) {
	type (
		nodeData struct {
			Status int                         `json:"status"`
			Data   clusterdump.NodeStatsBundle `json:"data"`
		}
		responseType struct {
			Status int                 `json:"status"`
			Nodes  map[string]nodeData `json:"nodes"`
		}
	)
	var t responseType
	ds := make(clusterdump.Stats)
	err := json.Unmarshal(b, &t)
	if err != nil {
		return ds, err
	}
	for k, v := range t.Nodes {
		ds[k] = v.Data
	}
	return ds, nil
}
