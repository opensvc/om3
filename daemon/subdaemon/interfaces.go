package subdaemon

import "context"

type (
	Manager interface {
		Running() bool
		Start(context.Context) error
		Stop() error
		MainStart(context.Context) error
		MainStop() error
		Name() string
		Register(Manager) error
	}

	RootManager interface {
		Running() bool
		Stop() error
	}
)
