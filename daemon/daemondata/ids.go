package daemondata

const (
	idUndef              = iota
	idApplyFull          = iota
	idApplyPatch         = iota
	idApplyPing          = iota
	idCommitPending      = iota
	idGetHbMessage       = iota
	idGetLocalNodeStatus = iota
	idGetServiceNames    = iota
	idGetStatus          = iota
	idPushOps            = iota
	idStats              = iota
)

var (
	idToName = map[int]string{
		idUndef:              "undef",
		idApplyFull:          "apply-full",
		idApplyPatch:         "apply-patch",
		idApplyPing:          "apply-ping",
		idCommitPending:      "commit-pending",
		idGetHbMessage:       "get-hb-message",
		idGetLocalNodeStatus: "get-local-node-status",
		idGetServiceNames:    "get-service-names",
		idGetStatus:          "get-status",
		idPushOps:            "push-ops",
		idStats:              "stats",
	}
)
