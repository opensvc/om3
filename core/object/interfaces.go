package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	// Configurer is implemented by object kinds supporting get, set, unset, eval, edit, ...
	Configurer interface {
		ConfigFile() string
		Config() *xconfig.T
		EditConfig() error
		RecoverAndEditConfig() error
		DiscardAndEditConfig() error
		RawConfig() (rawconfig.T, error)
		EvalConfig() (rawconfig.T, error)
		EvalConfigAs(string) (rawconfig.T, error)
		Eval(key.T) (interface{}, error)
		EvalAs(key.T, string) (interface{}, error)
		Get(key.T) (interface{}, error)
		ValidateConfig(context.Context) (xconfig.Alerts, error)
		DeleteSection(context.Context, ...string) error
		Delete(context.Context) error
		Set(context.Context, ...keyop.T) error
		Update(context.Context, []string, []key.T, []keyop.T) error
		Unset(context.Context, ...key.T) error
	}

	scheduler interface {
		Schedules() schedule.Table
	}
)
