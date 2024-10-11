package rescontainerpodman

import (
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/drivers/rescontainerocibase"
)

type (
	T struct {
		rescontainerocibase.BT
	}
)

func New() resource.Driver {
	return &T{
		BT: rescontainerocibase.BT{
			CProvider: &podman{},
		},
	}
}
