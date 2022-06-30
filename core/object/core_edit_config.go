package object

import (
	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/xconfig"
)

type OptsEditConfig struct {
	Discard bool `flag:"discard"`
	Recover bool `flag:"recover"`
}

func (t core) EditConfig(opts OptsEditConfig) error {
	var mode xconfig.EditMode
	switch {
	case opts.Discard && opts.Recover:
		return errors.New("discard and recover options are mutually exclusive")
	case opts.Discard:
		mode = xconfig.EditModeDiscard
	case opts.Recover:
		mode = xconfig.EditModeRecover
	default:
		mode = xconfig.EditModeNormal
	}
	return xconfig.Edit(t.ConfigFile(), mode, t.config.Referrer)
}
