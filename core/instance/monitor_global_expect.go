package instance

const (
	MonitorGlobalExpectInit MonitorGlobalExpect = iota
	MonitorGlobalExpectAborted
	MonitorGlobalExpectDeleted
	MonitorGlobalExpectFrozen
	MonitorGlobalExpectNone
	MonitorGlobalExpectPlaced
	MonitorGlobalExpectPlacedAt
	MonitorGlobalExpectProvisioned
	MonitorGlobalExpectPurged
	MonitorGlobalExpectRestarted
	MonitorGlobalExpectStarted
	MonitorGlobalExpectStopped
	MonitorGlobalExpectUnfrozen
	MonitorGlobalExpectUnprovisioned
)

var (
	MonitorGlobalExpectStrings map[MonitorGlobalExpect]string
	MonitorGlobalExpectValues  map[string]MonitorGlobalExpect
)

func init() {
	MonitorGlobalExpectStrings = make(map[MonitorGlobalExpect]string)
	MonitorGlobalExpectValues = make(map[string]MonitorGlobalExpect)

	expectStrings := []struct {
		expect MonitorGlobalExpect
		str    string
	}{
		{MonitorGlobalExpectAborted, "aborted"},
		{MonitorGlobalExpectDeleted, "deleted"},
		{MonitorGlobalExpectInit, "init"},
		{MonitorGlobalExpectFrozen, "frozen"},
		{MonitorGlobalExpectNone, "none"},
		{MonitorGlobalExpectPlaced, "placed"},
		{MonitorGlobalExpectPlacedAt, "placed@"},
		{MonitorGlobalExpectProvisioned, "provisioned"},
		{MonitorGlobalExpectPurged, "purged"},
		{MonitorGlobalExpectRestarted, "restarted"},
		{MonitorGlobalExpectStarted, "started"},
		{MonitorGlobalExpectStopped, "stopped"},
		{MonitorGlobalExpectUnfrozen, "unfrozen"},
		{MonitorGlobalExpectUnprovisioned, "unprovisioned"},
	}

	// Populate the maps
	for _, e := range expectStrings {
		MonitorGlobalExpectStrings[e.expect] = e.str
		MonitorGlobalExpectValues[e.str] = e.expect
	}
}
