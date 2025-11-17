package object

import (
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/file"
)

func (t *core) RecoverAndEditConfig() error {
	return file.Edit(t.ConfigFile(), file.EditModeRecover, func(dst string) error {
		return xconfig.ValidateReferrer(dst, t.config.Referrer)
	})
}

func (t *core) DiscardAndEditConfig() error {
	return file.Edit(t.ConfigFile(), file.EditModeDiscard, func(dst string) error {
		return xconfig.ValidateReferrer(dst, t.config.Referrer)
	})
}

func (t *core) EditConfig() error {
	return file.Edit(t.ConfigFile(), file.EditModeNormal, func(dst string) error {
		return xconfig.ValidateReferrer(dst, t.config.Referrer)
	})
}
