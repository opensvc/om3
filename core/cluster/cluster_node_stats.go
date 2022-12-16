package cluster

type (
	// NodeStats describes systems (cpu, mem, swap) resource usage of a node
	// and an opensvc-specific score.
	NodeStats struct {
		Load15M      float64 `json:"load_15m"`
		MemAvailPct  uint64  `json:"mem_avail"`
		MemTotalMB   uint64  `json:"mem_total"`
		Score        uint64  `json:"score"`
		SwapAvailPct uint64  `json:"swap_avail"`
		SwapTotalMB  uint64  `json:"swap_total"`
	}
)

func (n *NodeStats) DeepCopy() *NodeStats {
	var data NodeStats = *n
	return &data
}
