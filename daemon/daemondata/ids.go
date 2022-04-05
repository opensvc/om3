package daemondata

const (
	idUndef              = iota
	idStats              = iota
	idGetHbMessage       = iota
	idGetLocalNodeStatus = iota
	idGetStatus          = iota
	idApplyFull          = iota
	idApplyPatch         = iota
	idApplyPing          = iota
	idCommitPending      = iota
)

var (
	idToName = map[int]string{
		idUndef:              "undef",
		idStats:              "stats",
		idGetHbMessage:       "get-hb-message",
		idGetLocalNodeStatus: "get-local-node-status",
		idGetStatus:          "get-status",
		idApplyFull:          "apply-full",
		idApplyPatch:         "apply-patch",
		idApplyPing:          "apply-ping",
		idCommitPending:      "commit-pending",
	}
)
