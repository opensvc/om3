package object

import "opensvc.com/opensvc/config"

type (
	// Renderer is implemented by data type stored in ActionResults.Data.
	Renderer interface {
		Render() string
	}

	// Baser is implemented by all object kinds.
	Baser interface {
		Status(ActionOptionsStatus) (InstanceStatus, error)
	}

	// Starter is implemented by object kinds supporting start, stop, ...
	Starter interface {
		Start(ActionOptionsStart) error
		Stop(ActionOptionsStop) error
	}

	// Freezer is implemented by object kinds supporting freeze and thaw.
	Freezer interface {
		Freeze() error
		Unfreeze() error
		Thaw() error
	}

	// Configurer is implemented by object kinds supporting get, set, unset, eval, edit, ...
	Configurer interface {
		Config() *config.T
		Get(ActionOptionsGet) (interface{}, error)
		Set(ActionOptionsSet) error
	}
)
