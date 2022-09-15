package cluster

import (
	"encoding/json"
	"strings"
)

// MarshalJSON transforms a cluster.Status struct into a []byte
//func (t *Status) MarshalJSON()([]byte, error) {}

// UnmarshalJSON loads a byte array into a cluster.Status struct
func (t *Status) UnmarshalJSON(b []byte) error {
	var (
		m   map[string]interface{}
		ds  Status
		tmp []byte
		err error
	)
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	ds.Heartbeats = make(map[string]HeartbeatThreadStatus)

	for k, v := range m {
		tmp, err = json.Marshal(v)
		switch k {
		case "cluster":
			if err := json.Unmarshal(tmp, &ds.Cluster); err != nil {
				return err
			}
		case "monitor":
			if json.Unmarshal(tmp, &ds.Monitor); err != nil {
				return err
			}
		case "scheduler":
			if json.Unmarshal(tmp, &ds.Scheduler); err != nil {
				return err
			}
		case "collector":
			if json.Unmarshal(tmp, &ds.Collector); err != nil {
				return err
			}
		case "dns":
			if json.Unmarshal(tmp, &ds.DNS); err != nil {
				return err
			}
		case "listener":
			if json.Unmarshal(tmp, &ds.Listener); err != nil {
				return err
			}
		default:
			if strings.HasPrefix(k, "hb#") {
				var hb HeartbeatThreadStatus
				if err := json.Unmarshal(tmp, &hb); err != nil {
					return err
				}
				ds.Heartbeats[k] = hb
			}
		}
	}

	*t = ds
	return nil
}
