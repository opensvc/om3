package object

import (
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/schedule"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/key"
)

type (
	Enterer interface {
		Enter(OptsEnter) error
	}

	// Configurer is implemented by object kinds supporting get, set, unset, eval, edit, ...
	Configurer interface {
		ConfigFile() string
		Config() *xconfig.T
		EditConfig(OptsEditConfig) error
		PrintConfig(OptsPrintConfig) (rawconfig.T, error)
		ValidateConfig(OptsValidateConfig) (xconfig.ValidateAlerts, error)
		Eval(OptsEval) (interface{}, error)
		Get(OptsGet) (interface{}, error)
		Set(OptsSet) error
		Unset(OptsUnset) error
		Delete(OptsDelete) error
		SetKeys(kops ...keyop.T) error
		UnsetKeys(kws ...key.T) error
		Doc(OptsDoc) (string, error)
		SetKeywords(kws []string) error
	}

	scheduler interface {
		Schedules() schedule.Table
	}
)
