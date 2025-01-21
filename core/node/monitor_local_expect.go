package node

type (
	MonitorLocalExpect int
)

const (
	MonitorLocalExpectInit MonitorLocalExpect = iota
	MonitorLocalExpectDrained
	MonitorLocalExpectNone
)

var (
	StrMonitorLocalExpectInit    = "init"
	StrMonitorLocalExpectDrained = "drained"
	StrMonitorLocalExpectNone    = "none"

	MonitorLocalExpectStrings = map[MonitorLocalExpect]string{
		MonitorLocalExpectInit:    StrMonitorLocalExpectInit,
		MonitorLocalExpectDrained: StrMonitorLocalExpectDrained,
		MonitorLocalExpectNone:    StrMonitorLocalExpectNone,
	}

	MonitorLocalExpectValues = map[string]MonitorLocalExpect{
		StrMonitorLocalExpectInit:    MonitorLocalExpectInit,
		StrMonitorLocalExpectDrained: MonitorLocalExpectDrained,
		StrMonitorLocalExpectNone:    MonitorLocalExpectNone,
	}
)
