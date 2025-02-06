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
