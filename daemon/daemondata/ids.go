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
	idDelSmon
	idGetHbMessage
	idGetInstanceStatus
	idGetLocalNodeStatus
	idGetServiceNames
	idGetStatus
	idPushOps
	idSetInstanceConfig
	idSetInstanceStatus
	idSetServiceAgg
	idSetSmon
	idStats
)

var (
	idToName = map[int]string{
		idUndef:              "undef",
		idApplyFull:          "apply-full",
		idApplyPatch:         "apply-patch",
		idApplyPing:          "apply-ping",
		idCommitPending:      "commit-pending",
		idDelInstanceConfig:  "del-instance-config",
		idDelInstanceStatus:  "del-instance-status",
		idDelServiceAgg:      "del-service-agg",
		idDelSmon:            "del-smon",
		idGetHbMessage:       "get-hb-message",
		idGetInstanceStatus:  "get-instance-status",
		idGetLocalNodeStatus: "get-local-node-status",
		idGetServiceNames:    "get-service-names",
		idGetStatus:          "get-status",
		idPushOps:            "push-ops",
		idSetServiceAgg:      "set-service-agg",
		idSetInstanceConfig:  "set-instance-config",
		idSetInstanceStatus:  "set-instance-status",
		idSetSmon:            "set-smon",
		idStats:              "stats",
	}
)
