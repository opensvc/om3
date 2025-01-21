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

// Extracted constants for monitor state strings
const (
	StrInit        = "init"
	StrIdle        = "idle"
	StrRejoin      = "rejoin"
	StrMaintenance = "maintenance"
	StrUpgrade     = "upgrade"

	StrFreezeProgress = "freezing"
	StrFreezeFailure  = "freeze failed"
	StrFreezeSuccess  = "frozen"

	StrThawProgress = "thawing"
	StrThawFailure  = "thaw failed"
	StrThawSuccess  = "thawed"

	StrDrainProgress = "draining"
	StrDrainFailure  = "drain failed"
	StrDrainSuccess  = "drained"

	StrShutdownProgress = "shutting"
	StrShutdownFailure  = "shutdown failed"
	StrShutdownSuccess  = "shutdown"
)

var (
	// MonitorStateStrings is a map that associates MonitorState enums with
	// their corresponding string representations.
	MonitorStateStrings = map[MonitorState]string{
		MonitorStateInit:        StrInit,
		MonitorStateIdle:        StrIdle,
		MonitorStateMaintenance: StrMaintenance,
		MonitorStateRejoin:      StrRejoin,
		MonitorStateUpgrade:     StrUpgrade,

		MonitorStateFreezeProgress: StrFreezeProgress,
		MonitorStateFreezeFailure:  StrFreezeFailure,
		MonitorStateFreezeSuccess:  StrFreezeSuccess,

		MonitorStateThawProgress: StrThawProgress,
		MonitorStateThawFailure:  StrThawFailure,
		MonitorStateThawSuccess:  StrThawSuccess,

		MonitorStateDrainProgress: StrDrainProgress,
		MonitorStateDrainFailure:  StrDrainFailure,
		MonitorStateDrainSuccess:  StrDrainSuccess,

		MonitorStateShutdownProgress: StrShutdownProgress,
		MonitorStateShutdownFailure:  StrShutdownFailure,
		MonitorStateShutdownSuccess:  StrShutdownSuccess,
	}

	// MonitorStateValues maps string representations of various states to their
	// corresponding MonitorState constants.
	MonitorStateValues = map[string]MonitorState{
		StrInit:        MonitorStateInit,
		StrIdle:        MonitorStateIdle,
		StrRejoin:      MonitorStateRejoin,
		StrMaintenance: MonitorStateMaintenance,
		StrUpgrade:     MonitorStateUpgrade,

		StrFreezeProgress: MonitorStateFreezeProgress,
		StrFreezeFailure:  MonitorStateFreezeFailure,
		StrFreezeSuccess:  MonitorStateFreezeSuccess,

		StrThawProgress: MonitorStateThawProgress,
		StrThawFailure:  MonitorStateThawFailure,
		StrThawSuccess:  MonitorStateThawSuccess,

		StrDrainProgress: MonitorStateDrainProgress,
		StrDrainFailure:  MonitorStateDrainFailure,
		StrDrainSuccess:  MonitorStateDrainSuccess,

		StrShutdownProgress: MonitorStateShutdownProgress,
		StrShutdownFailure:  MonitorStateShutdownFailure,
		StrShutdownSuccess:  MonitorStateShutdownSuccess,
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
	}
)
