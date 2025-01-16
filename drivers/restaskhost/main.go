package restaskhost

// TODO
// * snooze
// * status.json rewrite after lock acquire

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/resapp"
	"github.com/opensvc/om3/drivers/restask"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/pg"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/proc"
	"github.com/opensvc/om3/util/ulimit"
)

// T is the driver structure.
type T struct {
	restask.BaseTask

	// From resapp.BaseT
	RetCodes   string         `json:"retcodes"`
	SecretsEnv []string       `json:"secret_environment"`
	ConfigsEnv []string       `json:"configs_environment"`
	Env        []string       `json:"environment"`
	Timeout    *time.Duration `json:"timeout"`
	//StartTimeout *time.Duration `json:"start_timeout"`
	StopTimeout *time.Duration `json:"stop_timeout"`
	Umask       *os.FileMode   `json:"umask"`
	ObjectID    uuid.UUID      `json:"objectID"`

	// From resapp.T
	Path    naming.Path   `json:"path"`
	Nodes   []string      `json:"nodes"`
	Cwd     string        `json:"cwd"`
	User    string        `json:"user"`
	Group   string        `json:"group"`
	PG      pg.Config     `json:"pg"`
	Limit   ulimit.Config `json:"limit"`
	StopCmd string

	RunCmd string
}

func New() resource.Driver {
	return &T{}
}

func (t *T) App() *resapp.T {
	return &resapp.T{
		BaseT: resapp.BaseT{
			T:           t.BaseTask.T,
			RetCodes:    t.RetCodes,
			Path:        t.Path,
			Nodes:       t.Nodes,
			SecretsEnv:  t.SecretsEnv,
			ConfigsEnv:  t.ConfigsEnv,
			Env:         t.Env,
			Timeout:     t.Timeout,
			Umask:       t.Umask,
			ObjectID:    t.ObjectID,
			StopTimeout: t.StopTimeout,
		},
		Path:    t.Path,
		Nodes:   t.Nodes,
		Cwd:     t.Cwd,
		User:    t.User,
		Group:   t.Group,
		PG:      t.PG,
		Limit:   t.Limit,
		StopCmd: t.StopCmd,
	}
}

func (t *T) Run(ctx context.Context) error {
	return t.RunIf(ctx, t.lockedRun)
}

func (t *T) loggerWithProc(p proc.T) *plog.Logger {
	return t.Log().Attr("cmd", p.CommandLine()).Attr("cmd_pid", p.PID())
}

func (t *T) loggerWithCmd(cmd *command.T) *plog.Logger {
	return t.Log().Attr("cmd", cmd.String())
}

func (t *T) lockedRun(ctx context.Context) (err error) {
	var opts []funcopt.O
	app := t.App()
	if opts, err = app.GetFuncOpts(t.RunCmd, "run"); err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}
	if t.LogOutputs {
		opts = append(opts,
			command.WithLogger(t.Log()),
			command.WithStdoutLogLevel(zerolog.InfoLevel),
			command.WithStderrLogLevel(zerolog.WarnLevel),
		)
	}
	opts = append(opts,
		command.WithTimeout(app.GetTimeout("run")),
		command.WithIgnoredExitCodes(),
	)
	cmd := command.New(opts...)
	t.loggerWithCmd(cmd).Infof("run %s", cmd)
	err = cmd.Run()
	if err := t.WriteLastRun(cmd.ExitCode()); err != nil {
		return err
	}
	if err != nil {
		t.Log().Errorf("write last run: %s", err)
		if err := t.onError(); err != nil {
			t.Log().Warnf("on error: %s", err)
		}
	}
	if s, err := t.BaseTask.ExitCodeToStatus(cmd.ExitCode()); err != nil {
		return err
	} else if s != status.Up {
		return fmt.Errorf("command exited with code %d", cmd.ExitCode())
	}
	return nil
}

func (t *T) onError() error {
	app := t.App()
	opts, err := app.GetFuncOpts(t.OnErrorCmd, "on_error")
	if err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}
	cmd := command.New(opts...)
	t.loggerWithCmd(cmd).Infof("on error run")
	return cmd.Run()
}

func (t *T) Kill(ctx context.Context) error {
	app := t.App()
	if app.StopCmd != "" {
		return app.CommonStop(ctx, t)
	}
	return t.stop(ctx)
}

func (t *T) stop(ctx context.Context) error {
	app := t.App()
	cmdArgs, err := app.BaseCmdArgs(app.StartCmd, "stop")
	if err != nil {
		return err
	}
	procs, err := t.getRunning(cmdArgs)
	if err != nil {
		return err
	}
	if procs.Len() == 0 {
		t.Log().Infof("already stopped")
		return nil
	}
	for _, p := range procs.Procs() {
		t.loggerWithProc(p).Infof("send termination signal to process %d", p.PID())
		p.Signal(syscall.SIGTERM)
	}
	prev := procs
	for i := 0; i < 5; i++ {
		procs, err := t.getRunning(cmdArgs)
		if err != nil {
			return err
		}
		for _, p := range prev.Procs() {
			if !procs.HasPID(p.PID()) {
				t.loggerWithProc(p).Infof("process %d is now terminated", p.PID())
			}
		}
		if procs.Len() == 0 {
			return nil
		}
		prev = procs
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("waited too long for process %s to disappear", procs)
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return ""
}

func (t *T) getRunning(cmdArgs []string) (proc.L, error) {
	procs, err := proc.All()
	if err != nil {
		return procs, err
	}
	procs = procs.FilterByEnv("OPENSVC_ID", t.ObjectID.String())
	procs = procs.FilterByEnv("OPENSVC_RID", t.RID())
	return procs, nil
}
