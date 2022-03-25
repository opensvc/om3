package daemondata

const (
	idUndef              = iota
	idStats              = iota
	idGetLocalNodeStatus = iota
	idGetStatus          = iota
	idApplyFull          = iota
	idApplyPatch         = iota
	idCommitPending      = iota
)

var (
	idToName = map[int]string{
		idUndef:              "undef",
		idStats:              "stats",
		idGetLocalNodeStatus: "get-local-node-status",
		idGetStatus:          "get-status",
		idApplyFull:          "apply-full",
		idApplyPatch:         "apply-patch",
		idCommitPending:      "commit-pending",
	}
)
