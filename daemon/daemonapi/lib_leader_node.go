package daemonapi

import (
	"fmt"

	"github.com/opensvc/om3/core/node"
)

func getLeaderNode() (string, error) {
	var leaderNode string
	for _, nStatus := range node.StatusData.GetAll() {
		if nStatus.Value.IsLeader {
			if leaderNode != "" {
				return "", fmt.Errorf("conflict: multiple leader nodes: %s, %s", leaderNode, nStatus.Node)
			}
			leaderNode = nStatus.Node
		}
	}
	if leaderNode == "" {
		return "", fmt.Errorf("missing leader node")
	}
	return leaderNode, nil
}
