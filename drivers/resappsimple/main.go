package resappsimple

import (
	"context"
	"opensvc.com/opensvc/core/actionrollback"

	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resapp"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/funcopt"
)

// T is the driver structure.
type T struct {
	resapp.T
	Kill string `json:"kill"`
}

func New() resource.Driver {
	return &T{}
}

func init() {
	resource.Register(driverGroup, driverName, New)
}

// Start the Resource
func (t T) Start(ctx context.Context) (err error) {
	t.Log().Debug().Msg("Start()")
	var opts []funcopt.O
	if opts, err = t.GetFuncOpts(t.StartCmd, "start"); err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}
	appStatus := t.Status()
	if appStatus == status.Up {
		t.Log().Info().Msg("already up")
		return nil
	}

	opts = append(opts, command.WithLogger(t.Log()))
	cmd := command.New(opts...)
	t.Log().Info().Msgf("running %s", cmd.String())
	err = cmd.Start()
	if err == nil {
		actionrollback.Register(ctx, func() error {
			return t.Stop(ctx)
		})
	}
	return
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return driverGroup.String()
}
