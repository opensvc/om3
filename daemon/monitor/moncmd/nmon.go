package moncmd

import "opensvc.com/opensvc/core/cluster"

type (
	SetNmon struct {
		Node    string
		Monitor cluster.NodeMonitor
	}

	NmonDeleted struct {
		Node string
	}

	NmonUpdated struct {
		Node    string
		Monitor cluster.NodeMonitor
	}
)
