package resappforking

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/resapp"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
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

func (t *T) Stop(ctx context.Context) error {
	if err := t.CommonStop(ctx, t); err != nil {
		// compat b2.1: ignore app resource stop error
		t.Log().Warnf("ignored stop failure: %s", err)
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	if t.CheckCmd == "" {
		t.StatusLog().Info("check is not set")
		return status.NotApplicable
	}
	return t.CommonStatus(ctx)
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return drvID.String()
}
