package subdaemon

type (
	Manager interface {
		Quit() error
		Running() bool
		Init() error
		Start() error
		Stop() error
		Name() string
		MainStart() error
		MainStop() error
		WaitDone()
	}
)
