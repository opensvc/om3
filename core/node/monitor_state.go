package node

type (
	MonitorState int
)

const (
	// Initialization and Miscellaneous states
	MonitorStateInit MonitorState = iota
	MonitorStateIdle
	MonitorStateRejoin
	MonitorStateMaintenance
	MonitorStateUpgrade

	// Freezing process states
	MonitorStateFreezeProgress
	MonitorStateFreezeFailure
	MonitorStateFreezeSuccess

	// Thawing process states
	MonitorStateThawProgress
	MonitorStateThawFailure
	MonitorStateThawSuccess

	// Draining process states
	MonitorStateDrainProgress
	MonitorStateDrainFailure
	MonitorStateDrainSuccess

	// Shutdown process states
	MonitorStateShutdownProgress
	MonitorStateShutdownFailure
	MonitorStateShutdownSuccess
)

var (
	// MonitorStateStrings is a map that associates MonitorState enums with
	// their corresponding string representations.
	MonitorStateStrings map[MonitorState]string

	// MonitorStateValues maps string representations of various states to their
	// corresponding MonitorState constants.
	MonitorStateValues map[string]MonitorState

	// MonitorStateUnrankable is the node monitor states evicting a node from ranking algorithms
	MonitorStateUnrankable = map[MonitorState]any{
		MonitorStateInit:             nil,
		MonitorStateRejoin:           nil,
		MonitorStateMaintenance:      nil,
		MonitorStateUpgrade:          nil,
		MonitorStateShutdownSuccess:  nil,
		MonitorStateShutdownFailure:  nil,
		MonitorStateShutdownProgress: nil,
	}
)

func init() {
	MonitorStateStrings = make(map[MonitorState]string)
	MonitorStateValues = make(map[string]MonitorState)

	stateStrings := []struct {
		state MonitorState
		str   string
	}{
		{MonitorStateInit, "init"},
		{MonitorStateIdle, "idle"},
		{MonitorStateMaintenance, "maintenance"},
		{MonitorStateRejoin, "rejoin"},
		{MonitorStateUpgrade, "upgrade"},
		{MonitorStateFreezeProgress, "freezing"},
		{MonitorStateFreezeFailure, "freeze failed"},
		{MonitorStateFreezeSuccess, "frozen"},
		{MonitorStateThawProgress, "thawing"},
		{MonitorStateThawFailure, "unfreeze failed"},
		{MonitorStateThawSuccess, "thawed"},
		{MonitorStateDrainProgress, "draining"},
		{MonitorStateDrainFailure, "drain failed"},
		{MonitorStateDrainSuccess, "drained"},
		{MonitorStateShutdownProgress, "shutting"},
		{MonitorStateShutdownFailure, "shutdown failed"},
		{MonitorStateShutdownSuccess, "shutdown"},
	}

	// Populate the maps
	for _, stateString := range stateStrings {
		MonitorStateStrings[stateString.state] = stateString.str
		MonitorStateValues[stateString.str] = stateString.state
	}
}
