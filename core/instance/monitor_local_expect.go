package instance

const (
	MonitorLocalExpectInit MonitorLocalExpect = iota
	MonitorLocalExpectNone
	MonitorLocalExpectStarted
	MonitorLocalExpectShutdown
	MonitorLocalExpectEvicted

	// MonitorLocalExpectUnknown is what a value this agent does not know decodes to:
	// a peer running a later version publishes values this one has no
	// name for, and failing to decode them failed the whole heartbeat
	// message the peer sent, which is how a peer is found dead.
	MonitorLocalExpectUnknown
)

var (
	monitorLocalExpectToString map[MonitorLocalExpect]string
	stringToMonitorLocalExpect map[string]MonitorLocalExpect
)

func init() {
	monitorLocalExpectToString = make(map[MonitorLocalExpect]string)
	stringToMonitorLocalExpect = make(map[string]MonitorLocalExpect)

	expectStrings := []struct {
		value MonitorLocalExpect
		str   string
	}{
		{MonitorLocalExpectEvicted, "evicted"},
		{MonitorLocalExpectStarted, "started"},
		{MonitorLocalExpectShutdown, "shutdown"},
		{MonitorLocalExpectNone, "none"},
		{MonitorLocalExpectInit, "init"},
		{MonitorLocalExpectUnknown, "unknown"},
	}

	// Populate the maps
	for _, e := range expectStrings {
		monitorLocalExpectToString[e.value] = e.str
		stringToMonitorLocalExpect[e.str] = e.value
	}
}
