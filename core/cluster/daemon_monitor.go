package cluster

type (

	// DaemonMonitor describes the OpenSVC daemon monitor thread state,
	// which is responsible for the node DataSets aggregation and
	// decision-making.
	DaemonMonitor struct {
		DaemonSubsystemStatus `yaml:",inline"`
	}
)
