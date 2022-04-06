package cluster

import "encoding/json"

type (
	// Status describes the full Cluster state.
	Status struct {
		Cluster    Info                             `json:"cluster"`
		Collector  CollectorThreadStatus            `json:"collector"`
		DNS        DNSThreadStatus                  `json:"dns"`
		Scheduler  SchedulerThreadStatus            `json:"scheduler"`
		Listener   ListenerThreadStatus             `json:"listener"`
		Monitor    MonitorThreadStatus              `json:"monitor"`
		Heartbeats map[string]HeartbeatThreadStatus `json:"-"`
	}

	// Info decribes the cluster id, name and nodes
	// The cluster name is used as the right most part of cluster dns
	// names.
	Info struct {
		ID    string   `json:"id"`
		Name  string   `json:"name"`
		Nodes []string `json:"nodes"`
	}
)

func (s *Status) DeepCopy() *Status {
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	newStatus := Status{}
	if err := json.Unmarshal(b, &newStatus); err != nil {
		return nil
	}
	return &newStatus
}
