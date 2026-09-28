package node

type (
	MonitorGlobalExpect int
)

const (
	MonitorGlobalExpectInit MonitorGlobalExpect = iota
	MonitorGlobalExpectAborted
	MonitorGlobalExpectFrozen
	MonitorGlobalExpectNone
	MonitorGlobalExpectUnfrozen

	// MonitorGlobalExpectUnknown is what a value this agent does not know decodes to:
	// a peer running a later version publishes values this one has no
	// name for, and failing to decode them failed the whole heartbeat
	// message the peer sent, which is how a peer is found dead.
	MonitorGlobalExpectUnknown
)

var (
	MonitorGlobalExpectStrings map[MonitorGlobalExpect]string
	MonitorGlobalExpectValues  map[string]MonitorGlobalExpect
)

func init() {
	MonitorGlobalExpectStrings = make(map[MonitorGlobalExpect]string)
	MonitorGlobalExpectValues = make(map[string]MonitorGlobalExpect)

	expectStrings := []struct {
		value MonitorGlobalExpect
		str   string
	}{
		{MonitorGlobalExpectAborted, "aborted"},
		{MonitorGlobalExpectFrozen, "frozen"},
		{MonitorGlobalExpectNone, "none"},
		{MonitorGlobalExpectUnfrozen, "unfrozen"},
		{MonitorGlobalExpectInit, "init"},
		{MonitorGlobalExpectUnknown, "unknown"},
	}

	// Populate the maps
	for _, e := range expectStrings {
		MonitorGlobalExpectStrings[e.value] = e.str
		MonitorGlobalExpectValues[e.str] = e.value
	}
}
