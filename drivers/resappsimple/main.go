package resappsimple

import (
	"context"

	"opensvc.com/opensvc/core/actionrollback"

	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resapp"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/proc"
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
	appStatus := t.Status(ctx)
	if appStatus == status.Up {
		t.Log().Info().Msg("already up")
		return nil
	}
	if err := t.ApplyPGChain(ctx); err != nil {
		return err
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

func (t *T) Status(ctx context.Context) status.T {
	if t.CheckCmd != "" {
		return t.CommonStatus(ctx)
	}
	return t.status()
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return driverGroup.String()
}

func (t *T) status() status.T {
	cmdArgs, err := t.CmdArgs(t.StartCmd, "start")
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	procs, err := t.getRunning(cmdArgs, true)
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	switch procs.Len() {
	case 0:
		return status.Down
	case 1:
		return status.Up
	default:
		t.StatusLog().Warn("too many process (%d)", procs.Len())
		return status.Up
	}
}

func (t T) getRunning(cmdArgs []string, withChildren bool) (*proc.L, error) {
	procs, err := proc.ByCmdline(cmdArgs)
	if err != nil {
		return procs, err
	}
	procs = procs.FilterByEnv("OPENSVC_ID", t.ObjectID.String())
	procs = procs.FilterByEnv("OPENSVC_RID", t.RID())
	return procs, nil
}
