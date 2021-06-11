package resappforking

import (
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resapp"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/xexec"
)

// T is the driver structure.
type T struct {
	resapp.T
}

func New() resource.Driver {
	return &T{}
}

func init() {
	resource.Register(driverGroup, driverName, New)
}

// Start the Resource
func (t T) Start() (err error) {
	t.Log().Debug().Msg("Start()")

	var opts []funcopt.O
	if opts, err = t.GetFuncOpts(t.StartCmd, "start"); err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}

	opts = append(opts,
		xexec.WithLogger(t.Log()),
		xexec.WithStdoutLogLevel(zerolog.InfoLevel),
		xexec.WithStderrLogLevel(zerolog.WarnLevel),
	)
	cmd := xexec.New(opts...)

	appStatus := t.Status()
	if appStatus == status.Up {
		t.Log().Info().Msg("already up")
		return nil
	}

	t.Log().Info().Msgf("runnning %s", cmd.String())
	return cmd.Run()
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return driverGroup.String()
}
