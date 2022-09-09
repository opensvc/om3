package moncmd

import (
	"time"

	"opensvc.com/opensvc/core/path"
)

type (
	Frozen struct {
		Path  path.T
		Node  string
		Value time.Time
	}
)
