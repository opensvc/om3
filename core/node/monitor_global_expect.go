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
		{MonitorGlobalExpectThawed, "thawed"},
		{MonitorGlobalExpectInit, "init"},
	}

	// Populate the maps
	for _, e := range expectStrings {
		MonitorGlobalExpectStrings[e.value] = e.str
		MonitorGlobalExpectValues[e.str] = e.value
	}
}
