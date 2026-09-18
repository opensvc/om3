package instance

const (
	// General States
	MonitorStateInit MonitorState = iota
	MonitorStateIdle

	MonitorStateBootSuccess
	MonitorStateBootFailed
	MonitorStateBootProgress

	MonitorStateShutdownProgress
	MonitorStateShutdownFailure
	MonitorStateShutdownSuccess

	MonitorStateStartProgress
	MonitorStateStartFailure
	MonitorStateStartSuccess

	MonitorStateStopProgress
	MonitorStateStopFailure
	MonitorStateStopSuccess

	MonitorStateFreezeProgress
	MonitorStateFreezeFailure
	MonitorStateFreezeSuccess

	MonitorStateUnfreezeProgress
	MonitorStateUnfreezeFailure
	MonitorStateUnfreezeSuccess

	MonitorStateProvisionProgress
	MonitorStateProvisionFailure
	MonitorStateProvisionSuccess

	MonitorStateUnprovisionProgress
	MonitorStateUnprovisionFailure
	MonitorStateUnprovisionSuccess

	MonitorStateDeleteProgress
	MonitorStateDeleteFailure
	MonitorStateDeleteSuccess

	MonitorStateResizeProgress
	MonitorStateResizeFailure
	MonitorStateResizeSuccess

	// MonitorStateResizeStage0 and its siblings are the states an instance
	// takes between the stages of a resize. A chain is grown in at most
	// MaxResizeStages stages, so they are named rather than counted.
	MonitorStateResizeStage0
	MonitorStateResizeStage1
	MonitorStateResizeStage2

	// wait states
	MonitorStateWaitChildren
	MonitorStateWaitParents
	MonitorStateWaitPriors
	MonitorStateWaitLeader
	MonitorStateWaitNonLeader

	// Miscellaneous
	MonitorStateRunning
	MonitorStateSyncing
	MonitorStatePurgeFailed
	MonitorStateReady
	MonitorStateRestarted
)

var (
	MonitorStateToString map[MonitorState]string
	StringToMonitorState map[string]MonitorState

	MonitorStatesFailure = []MonitorState{
		MonitorStateDeleteFailure,
		MonitorStateFreezeFailure,
		MonitorStateProvisionFailure,
		MonitorStateResizeFailure,
		MonitorStateShutdownFailure,
		MonitorStateStartFailure,
		MonitorStateStopFailure,
		MonitorStateUnfreezeFailure,
		MonitorStateUnprovisionFailure,
	}
)

func init() {
	MonitorStateToString = make(map[MonitorState]string)
	StringToMonitorState = make(map[string]MonitorState)

	stateStrings := []struct {
		state MonitorState
		str   string
	}{
		// General States
		{MonitorStateInit, "init"},
		{MonitorStateIdle, "idle"},

		{MonitorStateBootProgress, "booting"},
		{MonitorStateBootSuccess, "booted"},
		{MonitorStateBootFailed, "boot failed"},

		{MonitorStateShutdownProgress, "shutting"},
		{MonitorStateShutdownFailure, "shutdown failed"},
		{MonitorStateShutdownSuccess, "shutdown"},

		{MonitorStateStartProgress, "starting"},
		{MonitorStateStartFailure, "start failed"},
		{MonitorStateStartSuccess, "started"},

		{MonitorStateStopProgress, "stopping"},
		{MonitorStateStopFailure, "stop failed"},
		{MonitorStateStopSuccess, "stopped"},

		{MonitorStateFreezeProgress, "freezing"},
		{MonitorStateFreezeFailure, "freeze failed"},
		{MonitorStateFreezeSuccess, "frozen"},

		{MonitorStateUnfreezeProgress, "unfreezing"},
		{MonitorStateUnfreezeFailure, "unfreeze failed"},
		{MonitorStateUnfreezeSuccess, "unfrozen"},

		{MonitorStateProvisionProgress, "provisioning"},
		{MonitorStateProvisionFailure, "provision failed"},
		{MonitorStateProvisionSuccess, "provisioned"},

		{MonitorStateUnprovisionProgress, "unprovisioning"},
		{MonitorStateUnprovisionFailure, "unprovision failed"},
		{MonitorStateUnprovisionSuccess, "unprovisioned"},

		{MonitorStateDeleteProgress, "deleting"},
		{MonitorStateDeleteFailure, "delete failed"},
		{MonitorStateDeleteSuccess, "deleted"},

		{MonitorStateResizeProgress, "resizing"},
		{MonitorStateResizeFailure, "resize failed"},
		{MonitorStateResizeSuccess, "resized"},
		{MonitorStateResizeStage0, "resized:0"},
		{MonitorStateResizeStage1, "resized:1"},
		{MonitorStateResizeStage2, "resized:2"},

		// wait states
		{MonitorStateWaitChildren, "wait children"},
		{MonitorStateWaitParents, "wait parents"},
		{MonitorStateWaitLeader, "wait leader"},
		{MonitorStateWaitNonLeader, "wait non-leader"},
		{MonitorStateWaitPriors, "wait priors"},

		// Miscellaneous
		{MonitorStateRunning, "running"},
		{MonitorStateSyncing, "syncing"},
		{MonitorStatePurgeFailed, "purge failed"},
		{MonitorStateReady, "ready"},
		{MonitorStateRestarted, "restarted"},
	}

	// Populate the maps
	for _, stateString := range stateStrings {
		MonitorStateToString[stateString.state] = stateString.str
		StringToMonitorState[stateString.str] = stateString.state
	}
}

// MaxResizeStages is how many stages a resize orchestration grows a chain in.
// A chain crossing more replicated resources than that is grown by hand, one
// stage at a time.
const MaxResizeStages = 3

// ResizeStage is the stage an instance finished, and whether it is between
// stages at all.
func (t MonitorState) ResizeStage() (int, bool) {
	switch t {
	case MonitorStateResizeStage0:
		return 0, true
	case MonitorStateResizeStage1:
		return 1, true
	case MonitorStateResizeStage2:
		return 2, true
	default:
		return 0, false
	}
}

// NewMonitorStateResizeStage is the state of an instance that finished a
// stage of a resize, and whether there is one to name.
func NewMonitorStateResizeStage(stage int) (MonitorState, bool) {
	switch stage {
	case 0:
		return MonitorStateResizeStage0, true
	case 1:
		return MonitorStateResizeStage1, true
	case 2:
		return MonitorStateResizeStage2, true
	default:
		return MonitorStateInit, false
	}
}
