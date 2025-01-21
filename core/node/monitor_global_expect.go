package node

type (
	MonitorGlobalExpect int
)

const (
	MonitorGlobalExpectInit MonitorGlobalExpect = iota
	MonitorGlobalExpectAborted
	MonitorGlobalExpectFrozen
	MonitorGlobalExpectNone
	MonitorGlobalExpectThawed
)

const (
	StrMonitorGlobalExpectInit    = "init"
	StrMonitorGlobalExpectAborted = "aborted"
	StrMonitorGlobalExpectFrozen  = "frozen"
	StrMonitorGlobalExpectNone    = "none"
	StrMonitorGlobalExpectThawed  = "thawed"
)

var (
	MonitorGlobalExpectStrings = map[MonitorGlobalExpect]string{
		MonitorGlobalExpectAborted: StrMonitorGlobalExpectAborted,
		MonitorGlobalExpectFrozen:  StrMonitorGlobalExpectFrozen,
		MonitorGlobalExpectNone:    StrMonitorGlobalExpectNone,
		MonitorGlobalExpectThawed:  StrMonitorGlobalExpectThawed,
		MonitorGlobalExpectInit:    StrMonitorGlobalExpectInit,
	}

	MonitorGlobalExpectValues = map[string]MonitorGlobalExpect{
		StrMonitorGlobalExpectAborted: MonitorGlobalExpectAborted,
		StrMonitorGlobalExpectFrozen:  MonitorGlobalExpectFrozen,
		StrMonitorGlobalExpectNone:    MonitorGlobalExpectNone,
		StrMonitorGlobalExpectThawed:  MonitorGlobalExpectThawed,
		StrMonitorGlobalExpectInit:    MonitorGlobalExpectInit,
	}
)
