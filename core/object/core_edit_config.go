package object

import (
	"github.com/opensvc/om3/core/xconfig"
)

func (t core) RecoverAndEditConfig() error {
	return xconfig.Edit(t.ConfigFile(), xconfig.EditModeRecover, t.config.Referrer)
}

func (t core) DiscardAndEditConfig() error {
	return xconfig.Edit(t.ConfigFile(), xconfig.EditModeDiscard, t.config.Referrer)
}

func (t core) EditConfig() error {
	return xconfig.Edit(t.ConfigFile(), xconfig.EditModeNormal, t.config.Referrer)
}
