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
	MonitorGlobalExpectResized
	MonitorGlobalExpectRestarted
	MonitorGlobalExpectStarted
	MonitorGlobalExpectStopped
	MonitorGlobalExpectUnfrozen
	MonitorGlobalExpectUnprovisioned
	MonitorGlobalExpectCapped

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
		expect MonitorGlobalExpect
		str    string
	}{
		{MonitorGlobalExpectAborted, "aborted"},
		{MonitorGlobalExpectCapped, "capped"},
		{MonitorGlobalExpectDeleted, "deleted"},
		{MonitorGlobalExpectInit, "init"},
		{MonitorGlobalExpectFrozen, "frozen"},
		{MonitorGlobalExpectNone, "none"},
		{MonitorGlobalExpectPlaced, "placed"},
		{MonitorGlobalExpectPlacedAt, "placed@"},
		{MonitorGlobalExpectProvisioned, "provisioned"},
		{MonitorGlobalExpectPurged, "purged"},
		{MonitorGlobalExpectResized, "resized"},
		{MonitorGlobalExpectRestarted, "restarted"},
		{MonitorGlobalExpectStarted, "started"},
		{MonitorGlobalExpectStopped, "stopped"},
		{MonitorGlobalExpectUnfrozen, "unfrozen"},
		{MonitorGlobalExpectUnprovisioned, "unprovisioned"},
		{MonitorGlobalExpectUnknown, "unknown"},
	}

	// Populate the maps
	for _, e := range expectStrings {
		MonitorGlobalExpectStrings[e.expect] = e.str
		MonitorGlobalExpectValues[e.str] = e.expect
	}
}
