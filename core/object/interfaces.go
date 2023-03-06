package object

import (
	"context"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/schedule"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/key"
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
		Doc(string, string) (string, error)
		DriverDoc(string) (string, error)
		KeywordDoc(string) (string, error)
	}

	scheduler interface {
		Schedules() schedule.Table
	}
)
