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
	}

	// Populate the maps
	for _, e := range expectStrings {
		MonitorLocalExpectStrings[e.value] = e.str
		MonitorLocalExpectValues[e.str] = e.value
	}
}
