package object

import (
	"github.com/opensvc/om3/core/xconfig"
)

func (t Node) RecoverAndEditConfig() error {
	return xconfig.Edit(t.ConfigFile(), xconfig.EditModeRecover, t.config.Referrer)
}

func (t Node) DiscardAndEditConfig() error {
	return xconfig.Edit(t.ConfigFile(), xconfig.EditModeDiscard, t.config.Referrer)
}

func (t Node) EditConfig() error {
	return xconfig.Edit(t.ConfigFile(), xconfig.EditModeNormal, t.config.Referrer)
}
