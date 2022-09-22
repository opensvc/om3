package cluster

type (

	// MonitorThreadStatus describes the OpenSVC daemon monitor thread state,
	// which is responsible for the node DataSets aggregation and
	// decision-making.
	MonitorThreadStatus struct {
		ThreadStatus
		Routines int `json:"routines"`
	}
)
