package instance

const (
	MonitorLocalExpectInit MonitorLocalExpect = iota
	MonitorLocalExpectNone
	MonitorLocalExpectStarted
	MonitorLocalExpectShutdown
	MonitorLocalExpectEvicted
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
	}

	// Populate the maps
	for _, e := range expectStrings {
		monitorLocalExpectToString[e.value] = e.str
		stringToMonitorLocalExpect[e.str] = e.value
	}
}
