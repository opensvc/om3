package object

import (
	"context"

	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/schedule"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/key"
)

type (
	// Configurer is implemented by object kinds supporting get, set, unset, eval, edit, ...
	Configurer interface {
		ConfigFile() string
		Config() *xconfig.T
		EditConfig() error
		RecoverAndEditConfig() error
		DiscardAndEditConfig() error
		PrintConfig() (rawconfig.T, error)
		EvalConfig() (rawconfig.T, error)
		EvalConfigAs(string) (rawconfig.T, error)
		Eval(key.T) (interface{}, error)
		EvalAs(key.T, string) (interface{}, error)
		Get(key.T) (interface{}, error)
		ValidateConfig(context.Context) (xconfig.ValidateAlerts, error)
		DeleteSection(context.Context, string) error
		Delete(context.Context) error
		Set(context.Context, ...keyop.T) error
		Unset(context.Context, ...key.T) error
		DriverDoc(string) (string, error)
		KeywordDoc(string) (string, error)
	}

	scheduler interface {
		Schedules() schedule.Table
	}
)
