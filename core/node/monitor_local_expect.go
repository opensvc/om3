package node

type (
	MonitorLocalExpect int
)

const (
	MonitorLocalExpectInit MonitorLocalExpect = iota
	MonitorLocalExpectDrained
	MonitorLocalExpectNone

	// MonitorLocalExpectUnknown is what a value this agent does not know decodes to:
	// a peer running a later version publishes values this one has no
	// name for, and failing to decode them failed the whole heartbeat
	// message the peer sent, which is how a peer is found dead.
	MonitorLocalExpectUnknown
)

var (
	MonitorLocalExpectStrings map[MonitorLocalExpect]string
	MonitorLocalExpectValues  map[string]MonitorLocalExpect
)

func init() {
	MonitorLocalExpectStrings = make(map[MonitorLocalExpect]string)
	MonitorLocalExpectValues = make(map[string]MonitorLocalExpect)

	expectStrings := []struct {
		value MonitorLocalExpect
		str   string
	}{
		{MonitorLocalExpectInit, "init"},
		{MonitorLocalExpectDrained, "drained"},
		{MonitorLocalExpectNone, "none"},
		{MonitorLocalExpectUnknown, "unknown"},
	}

	// Populate the maps
	for _, e := range expectStrings {
		MonitorLocalExpectStrings[e.value] = e.str
		MonitorLocalExpectValues[e.str] = e.value
	}
}
