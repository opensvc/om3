package plugins

import (
	"github.com/opensvc/om3/v3/core/driver"
)

type (
	Factory struct {
		m map[driver.ID]any
	}
)

func NewFactory() Factory {
	return Factory{
		m: make(map[driver.ID]any),
	}
}

func (t *Factory) Register(driverID driver.ID, builder any) {
	t.m[driverID] = builder
}

func (t *Factory) Dump() map[driver.ID]any {
	return t.m
}
