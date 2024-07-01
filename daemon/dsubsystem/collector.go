package dsubsystem

type (
	// Collector describes the OpenSVC daemon collector subsystem,
	// which is responsible for communicating with the collector on behalf
	// of the cluster. Only one node on the cluster is the collector speaker.
	Collector struct {
		DaemonSubsystemStatus
	}
)

func (c *Collector) DeepCopy() *Collector {
	return &Collector{
		DaemonSubsystemStatus: *c.DaemonSubsystemStatus.DeepCopy(),
	}
}
