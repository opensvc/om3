package moncmd

import (
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
)

type (
	InstStatusDeleted struct {
		Path path.T
		Node string
	}

	InstStatusUpdated struct {
		Path   path.T
		Node   string
		Status instance.Status
	}
)
