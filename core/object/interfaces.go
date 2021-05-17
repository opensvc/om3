package object

import (
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceset"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	// Renderer is implemented by data type stored in ActionResults.Data.
	Renderer interface {
		Render() string
	}

	// Baser is implemented by all object kinds.
	Baser interface {
		Status(OptsStatus) (instance.Status, error)
		Exists() bool
		IsVolatile() bool
		ResourceSets() resourceset.L
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
		Exists() bool
		ConfigFile() string
		Config() *config.T
		EditConfig(OptsEditConfig) error
		Eval(OptsEval) (interface{}, error)
		Get(OptsGet) (interface{}, error)
		Set(OptsSet) error
		Unset(OptsUnset) error
		Delete(OptsDelete) error
		SetStandardConfigFile()
	}

	// ResourceLister provides a method to list and filter resources
	ResourceLister interface {
		Resources() resource.Drivers
		IsDesc() bool
	}
)
