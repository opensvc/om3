package object

import (
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	// Renderer is implemented by data type stored in ActionResults.Data.
	Renderer interface {
		Render() string
	}

	// Baser is implemented by all object kinds.
	Baser interface {
		Status(OptsStatus) (InstanceStatus, error)
		Exists() bool
	}

	// Starter is implemented by object kinds supporting start, stop, ...
	Starter interface {
		Start(OptsStart) error
		Stop(OptsStop) error
	}

	// Freezer is implemented by object kinds supporting freeze and thaw.
	Freezer interface {
		Freeze() error
		Unfreeze() error
		Thaw() error
		Frozen() timestamp.T
	}

	// Configurer is implemented by object kinds supporting get, set, unset, eval, edit, ...
	Configurer interface {
		ConfigFile() string
		Config() *config.T
		Get(OptsGet) (interface{}, error)
		Set(OptsSet) error
	}
)
