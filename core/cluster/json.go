package cluster

import (
	"encoding/json"
	"strings"
)

// MarshalJSON transforms a cluster.Status struct into a []byte
//func (t *Status) MarshalJSON()([]byte, error) {}

// UnmarshalJSON loads a byte array into a cluster.Status struct
func (t *Status) UnmarshalJSON(b []byte) error {
	var m map[string]interface{}
	err := json.Unmarshal(b, &m)
	if err != nil {
		return err
	}
	var ds Status
	var tmp []byte
	ds.Heartbeats = make(map[string]HeartbeatThreadStatus)

	for k, v := range m {
		tmp, err = json.Marshal(v)
		switch k {
		case "cluster":
			json.Unmarshal(tmp, &ds.Cluster)
		case "monitor":
			json.Unmarshal(tmp, &ds.Monitor)
		case "scheduler":
			json.Unmarshal(tmp, &ds.Scheduler)
		case "collector":
			json.Unmarshal(tmp, &ds.Collector)
		case "dns":
			json.Unmarshal(tmp, &ds.DNS)
		case "listener":
			json.Unmarshal(tmp, &ds.Listener)
		default:
			if strings.HasPrefix(k, "hb#") {
				var hb HeartbeatThreadStatus
				json.Unmarshal(tmp, &hb)
				ds.Heartbeats[k] = hb
			}
		}
	}

	*t = ds
	return nil
}
