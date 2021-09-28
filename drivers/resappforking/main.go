package resappforking

import (
	"context"

	"github.com/rs/zerolog"
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
	if err := t.ApplyPGChain(ctx); err != nil {
		return err
	}

	opts = append(opts,
		command.WithLogger(t.Log()),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.WarnLevel),
		command.WithTimeout(t.GetTimeout("start")),
	)
	cmd := command.New(opts...)

	appStatus := t.Status(ctx)
	if appStatus == status.Up {
		t.Log().Info().Msg("already up")
		return nil
	}

	t.Log().Info().Stringer("cmd", cmd).Msg("run")
	err = cmd.Run()
	if err == nil {
		actionrollback.Register(ctx, func() error {
			return t.Stop(ctx)
		})
	}
	return
}

func (t *T) Stop(ctx context.Context) error {
	return t.CommonStop(ctx, t)
}

func (t *T) Status(ctx context.Context) status.T {
	if t.CheckCmd == "" {
		t.StatusLog().Info("check is not set")
		return status.NotApplicable
	}
	return t.CommonStatus(ctx)
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return driverGroup.String()
}
