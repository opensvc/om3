package moncmd

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
)

type (
	MonSvcAggDeleted struct {
		Path path.T
		Node string
	}

	MonSvcAggUpdated struct {
		Path   path.T
		Node   string
		SvcAgg object.AggregatedStatus
		SrcEv  *T
	}

	MonSvcAggDone struct {
		Path path.T
	}
)
