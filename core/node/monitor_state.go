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

	// Unfreezing process states
	MonitorStateUnfreezeProgress
	MonitorStateUnfreezeFailure
	MonitorStateUnfreezeSuccess

	// Draining process states
	MonitorStateDrainProgress
	MonitorStateDrainFailure
	MonitorStateDrainSuccess

	// Shutdown process states
	MonitorStateShutdownProgress
	MonitorStateShutdownFailure
	MonitorStateShutdownSuccess

	// Upgrade of the opensvc packages outcomes: the upgrade itself runs in
	// the MonitorStateUpgrade state.
	MonitorStateUpgradeFailure
	MonitorStateUpgradeSuccess

	// MonitorStateUnknown is what a value this agent does not know decodes to:
	// a peer running a later version publishes values this one has no
	// name for, and failing to decode them failed the whole heartbeat
	// message the peer sent, which is how a peer is found dead.
	MonitorStateUnknown
)

var (
	// MonitorStateStrings is a map that associates MonitorState enums with
	// their corresponding string representations.
	MonitorStateStrings map[MonitorState]string

	// MonitorStateValues maps string representations of various states to their
	// corresponding MonitorState constants.
	MonitorStateValues map[string]MonitorState

	// MonitorStatesFailure is the node monitor states an orchestration ends
	// on when it gave up rather than reached what it was for.
	MonitorStatesFailure = []MonitorState{
		MonitorStateDrainFailure,
		MonitorStateFreezeFailure,
		MonitorStateShutdownFailure,
		MonitorStateUnfreezeFailure,
		MonitorStateUpgradeFailure,
	}

	// MonitorStatesAway is the node monitor states of a node away, its
	// daemon stopped, starting, rejoining or shutting down: it runs no
	// orchestration, and keeps the data of its instances in the cluster,
	// which the orchestrations of its peers do not wait for. A node
	// rejoining adopts the orchestrations running, and catches up with the
	// ones it missed.
	MonitorStatesAway = []MonitorState{
		MonitorStateInit,
		MonitorStateRejoin,
		MonitorStateMaintenance,
		MonitorStateShutdownProgress,
		MonitorStateShutdownSuccess,
		MonitorStateShutdownFailure,
	}

	// MonitorStateUnrankable is the node monitor states evicting a node from ranking algorithms
	MonitorStateUnrankable = map[MonitorState]any{
		MonitorStateInit:             nil,
		MonitorStateRejoin:           nil,
		MonitorStateMaintenance:      nil,
		MonitorStateUpgrade:          nil,
		MonitorStateShutdownSuccess:  nil,
		MonitorStateShutdownFailure:  nil,
		MonitorStateShutdownProgress: nil,
		// A node in a state this agent does not know is not one to place
		// an instance on.
		MonitorStateUnknown: nil,
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
		{MonitorStateUnfreezeProgress, "unfreezing"},
		{MonitorStateUnfreezeFailure, "unfreeze failed"},
		{MonitorStateUnfreezeSuccess, "unfrozen"},
		{MonitorStateDrainProgress, "draining"},
		{MonitorStateDrainFailure, "drain failed"},
		{MonitorStateDrainSuccess, "drained"},
		{MonitorStateShutdownProgress, "shutting"},
		{MonitorStateShutdownFailure, "shutdown failed"},
		{MonitorStateShutdownSuccess, "shutdown"},
		{MonitorStateUpgradeFailure, "upgrade failed"},
		{MonitorStateUpgradeSuccess, "upgraded"},
		{MonitorStateUnknown, "unknown"},
	}

	// Populate the maps
	for _, stateString := range stateStrings {
		MonitorStateStrings[stateString.state] = stateString.str
		MonitorStateValues[stateString.str] = stateString.state
	}
}

// IsAway says the node of this monitor state is away: see MonitorStatesAway.
func (t MonitorState) IsAway() bool {
	return t.IsOneOf(MonitorStatesAway...)
}

// IsAwayFromObjects says the node of this monitor state runs no object
// orchestration: away, or upgrading its packages, which restarts its daemon.
// The node orchestrations, an upgrade among them, judge with IsAway.
func (t MonitorState) IsAwayFromObjects() bool {
	return t.IsAway() || t.IsMaintenance()
}
