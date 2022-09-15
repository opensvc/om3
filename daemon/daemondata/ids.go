package daemondata

const (
	idUndef = iota
	idApplyFull
	idApplyPatch
	idApplyPing
	idCommitPending
	idDelInstanceConfig
	idDelInstanceStatus
	idDelServiceAgg
	idDelNmon
	idDelSmon
	idGetHbMessage
	idGetInstanceStatus
	idGetNodeStatus
	idGetServiceNames
	idGetNmon
	idGetStatus
	idSetInstanceConfig
	idSetInstanceStatus
	idSetServiceAgg
	idSetNmon
	idSetSmon
	idStats
)

var (
	idToName = map[int]string{
		idUndef:             "undef",
		idApplyFull:         "apply-full",
		idApplyPatch:        "apply-patch",
		idApplyPing:         "apply-ping",
		idCommitPending:     "commit-pending",
		idDelInstanceConfig: "del-instance-config",
		idDelInstanceStatus: "del-instance-status",
		idDelServiceAgg:     "del-service-agg",
		idDelNmon:           "del-nmon",
		idDelSmon:           "del-smon",
		idGetHbMessage:      "get-hb-message",
		idGetInstanceStatus: "get-instance-status",
		idGetNodeStatus:     "get-node-status",
		idGetServiceNames:   "get-service-names",
		idGetStatus:         "get-status",
		idSetServiceAgg:     "set-service-agg",
		idSetInstanceConfig: "set-instance-config",
		idSetInstanceStatus: "set-instance-status",
		idSetNmon:           "set-nmon",
		idSetSmon:           "set-smon",
		idStats:             "stats",
	}
)
