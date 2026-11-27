package resappforking

import (
	"context"
	"errors"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resapp"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/sdunit"
)

// T is the driver structure.
type T struct {
	resapp.T
}

func New() resource.Driver {
	return &T{}
}

func (t *T) loggerWithCmd(cmd *command.T) *plog.Logger {
	return t.Log().Attr("cmd", cmd.String())
}

// Start the Resource
func (t *T) Start(ctx context.Context) (err error) {
	if t.UnitMode() {
		return t.startUnit(ctx)
	}
	var opts []funcopt.O
	if opts, err = t.GetFuncOpts(ctx, t.StartCmd, "start"); err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}

	opts = append(opts,
		command.WithLogger(t.Log()),
		command.WithErrorExitCodeLogLevel(zerolog.WarnLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.WarnLevel),
		command.WithTimeout(t.GetTimeout("start")),
	)
	cmd := command.New(opts...)

	appStatus := t.Status(ctx)
	if appStatus == status.Up {
		t.Log().Infof("already up")
		return nil
	}

	t.loggerWithCmd(cmd).Infof("run: %s", cmd)
	err = cmd.Run()
	if err == nil {
		actionrollback.Register(ctx, func(ctx context.Context) error {
			return t.Stop(ctx)
		})
	}
	return
}

// startUnit runs the start command in the transient service unit of the
// resource, of type forking: systemd-run returns once the command returned,
// the processes it forked left running in the unit. Its output goes to the
// journal of the unit, and is logged from there.
func (t *T) startUnit(ctx context.Context) error {
	if t.Status(ctx) == status.Up {
		t.Log().Infof("already up")
		return nil
	}
	opts, cleanup, err := t.UnitRun(ctx, t.StartCmd, "start", sdunit.Run{
		Type:          "forking",
		OwnProperties: []sdunit.Property{t.UnitStartTimeout()},
	})
	defer cleanup()
	if err != nil || len(opts) == 0 {
		return err
	}
	opts = append(opts,
		command.WithLogger(t.Log()),
		command.WithErrorExitCodeLogLevel(zerolog.WarnLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.WarnLevel),
	)
	cmd := command.New(opts...)
	t.loggerWithCmd(cmd).Infof("run: %s", cmd)
	if err := cmd.Run(); err != nil {
		return t.UnitFailure(ctx, err)
	}
	t.UnitLogJournal(ctx)
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.Stop(ctx)
	})
	return nil
}

// Stop runs the stop command, then, for a start command run in a unit,
// ends the processes it left in the unit.
func (t *T) Stop(ctx context.Context) error {
	// Read before the stop ends what the unit runs.
	inUnit := t.UnitHolds(ctx)
	err := t.CommonStop(ctx, t)
	if inUnit {
		err = errors.Join(err, t.UnitTerminate(ctx))
		t.NetClear()
	}
	if err != nil {
		if !actioncontext.HasResourceSelector(ctx) {
			// compat b2.1: ignore app resource stop error
			t.Log().Warnf("ignored stop failure: %s", err)
			return nil
		} else {
			return err
		}
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	if t.CheckCmd == "" {
		t.StatusLog().Info("check is not set")
		return status.NotApplicable
	}
	s := t.CommonStatus(ctx)
	if s == status.Up {
		t.NetStatus()
	}
	return s
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.Desc
}
