package restaskdocker

// TODO
// * snooze
// * status.json rewrite after lock acquire

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/mattn/go-isatty"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/rescontainerdocker"
	"github.com/opensvc/om3/util/confirmation"
	"github.com/opensvc/om3/util/pg"
	"github.com/opensvc/om3/util/retcodes"
)

// T is the driver structure.
type T struct {
	resource.T
	resource.SCSIPersistentReservation
	Detach          bool           `json:"detach"`
	PG              pg.Config      `json:"pg"`
	Path            naming.Path    `json:"path"`
	ObjectID        uuid.UUID      `json:"object_id"`
	SCSIReserv      bool           `json:"scsireserv"`
	PromoteRW       bool           `json:"promote_rw"`
	NoPreemptAbort  bool           `json:"no_preempt_abort"`
	OsvcRootPath    string         `json:"osvc_root_path"`
	GuestOS         string         `json:"guest_os"`
	Name            string         `json:"name"`
	Nodes           []string       `json:"nodes"`
	Hostname        string         `json:"hostname"`
	Image           string         `json:"image"`
	ImagePullPolicy string         `json:"image_pull_policy"`
	CWD             string         `json:"cwd"`
	User            string         `json:"user"`
	Command         []string       `json:"command"`
	DNS             []string       `json:"dns"`
	DNSSearch       []string       `json:"dns_search"`
	RunArgs         []string       `json:"run_args"`
	Entrypoint      []string       `json:"entrypoint"`
	Remove          bool           `json:"remove"`
	Privileged      bool           `json:"privileged"`
	Init            bool           `json:"init"`
	Interactive     bool           `json:"interactive"`
	TTY             bool           `json:"tty"`
	VolumeMounts    []string       `json:"volume_mounts"`
	Env             []string       `json:"environment"`
	SecretsEnv      []string       `json:"secrets_environment"`
	ConfigsEnv      []string       `json:"configs_environment"`
	Devices         []string       `json:"devices"`
	NetNS           string         `json:"netns"`
	UserNS          string         `json:"userns"`
	PIDNS           string         `json:"pidns"`
	IPCNS           string         `json:"ipcns"`
	UTSNS           string         `json:"utsns"`
	RegistryCreds   string         `json:"registry_creds"`
	PullTimeout     *time.Duration `json:"pull_timeout"`
	Timeout         *time.Duration `json:"timeout"`

	RetCodes string `json:"retcodes"`

	Check        string
	Schedule     string
	Confirmation bool
	LogOutputs   bool
	Snooze       *time.Duration
}

const (
	lockName = "run"
)

func New() resource.Driver {
	return &T{}
}

func (t T) Container() *rescontainerdocker.T {
	return &rescontainerdocker.T{
		T:                         t.T,
		Detach:                    false,
		SCSIPersistentReservation: t.SCSIPersistentReservation,
		PG:                        t.PG,
		Path:                      t.Path,
		ObjectID:                  t.ObjectID,
		SCSIReserv:                t.SCSIReserv,
		PromoteRW:                 t.PromoteRW,
		NoPreemptAbort:            t.NoPreemptAbort,
		OsvcRootPath:              t.OsvcRootPath,
		GuestOS:                   t.GuestOS,
		Name:                      t.Name,
		Hostname:                  t.Hostname,
		Image:                     t.Image,
		ImagePullPolicy:           t.ImagePullPolicy,
		CWD:                       t.CWD,
		User:                      t.User,
		Command:                   t.Command,
		DNS:                       t.DNS,
		DNSSearch:                 t.DNSSearch,
		RunArgs:                   t.RunArgs,
		Entrypoint:                t.Entrypoint,
		Remove:                    t.Remove,
		Privileged:                t.Privileged,
		Init:                      t.Init,
		Interactive:               t.Interactive,
		TTY:                       t.TTY,
		VolumeMounts:              t.VolumeMounts,
		Env:                       t.Env,
		SecretsEnv:                t.SecretsEnv,
		ConfigsEnv:                t.ConfigsEnv,
		Devices:                   t.Devices,
		NetNS:                     t.NetNS,
		UserNS:                    t.UserNS,
		PIDNS:                     t.PIDNS,
		IPCNS:                     t.IPCNS,
		UTSNS:                     t.UTSNS,
		RegistryCreds:             t.RegistryCreds,
		PullTimeout:               t.PullTimeout,
		StartTimeout:              t.Timeout,
	}
}

func (t T) IsRunning() bool {
	unlock, err := t.Lock(false, time.Second*0, lockName)
	if err != nil {
		return true
	}
	defer unlock()
	return false
}

func (t T) Run(ctx context.Context) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	unlock, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedRun(ctx)
}

func (t T) ExitCodeToStatus(exitCode int) (status.T, error) {
	m, err := retcodes.Parse(t.RetCodes)
	if err != nil {
		return status.Warn, err
	}
	return m.Status(exitCode), nil
}

func (t T) lockedRun(ctx context.Context) (err error) {
	if !env.HasDaemonOrigin() {
		defer t.notifyRunDone()
	}
	if err := t.handleConfirmation(ctx); err != nil {
		return err
	}
	if err := t.ApplyPGChain(ctx); err != nil {
		return err
	}
	// TODO: if t.LogOutputs {}
	container := t.Container()
	if err := container.Start(ctx); err != nil {
		t.Log().Errorf("%s", err)
		return err
	}
	inspect, err := container.Inspect(ctx)
	if err != nil {
		return err
	}
	if err := t.writeLastRun(inspect.State.ExitCode); err != nil {
		t.Log().Errorf("write last run: %s", err)
		return err
	}
	if s, err := t.ExitCodeToStatus(inspect.State.ExitCode); err != nil {
		return err
	} else if s != status.Up {
		/* ? TODO:
		if err := t.onError(); err != nil {
			t.Log().Warnf("on error: %s", err)
		}
		*/
		return fmt.Errorf("command exited with code %d", inspect.State.ExitCode)
	}
	return nil
}

/*
func (t T) onError() error {
	opts, err := t.GetFuncOpts(t.OnErrorCmd, "on_error")
	if err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}
	cmd := command.New(opts...)
	t.Log().Infof("on error run")
	return cmd.Run()
}
*/

func (t *T) Kill(ctx context.Context) error {
	return t.Container().Signal(syscall.SIGKILL)
}

func (t *T) Status(ctx context.Context) status.T {
	switch t.Check {
	case "last_run":
		return t.statusLastRun(ctx)
	default:
		return status.NotApplicable
	}
}

func (t T) writeLastRun(retcode int) error {
	p := t.lastRunFile()
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintf(f, "%d\n", retcode)
	return nil
}

func (t T) readLastRun() (int, error) {
	p := t.lastRunFile()
	if b, err := os.ReadFile(p); err != nil {
		return 0, err
	} else {
		return strconv.Atoi(strings.TrimSpace(string(b)))
	}
}

func (t T) lastRunFile() string {
	return filepath.Join(t.VarDir(), "last_run_retcode")
}

func (t *T) statusLastRun(ctx context.Context) status.T {
	if err := resource.StatusCheckRequires(ctx, t); err != nil {
		t.StatusLog().Info("requirements not met")
		return status.NotApplicable
	}
	if i, err := t.readLastRun(); err != nil {
		t.StatusLog().Info("never run")
		return status.NotApplicable
	} else {
		s, err := t.ExitCodeToStatus(i)
		if err != nil {
			t.StatusLog().Info("%s", err)
		}
		if s != status.Up {
			t.StatusLog().Info("last run failed (%d)", i)
		}
		return s
	}
}

func (t *T) running(ctx context.Context) bool {
	inspect, err := t.Container().Inspect(ctx)
	if err != nil {
		return false
	}
	return inspect.State.Running
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return ""
}

func (t T) handleConfirmation(ctx context.Context) error {
	if !t.Confirmation {
		return nil
	}
	if actioncontext.IsConfirm(ctx) {
		t.Log().Infof("run confirmed by --confirm command line option")
		return nil
	}
	if actioncontext.IsCron(ctx) {
		// as set by the daemon scheduler subsystem
		return fmt.Errorf("run aborted (--cron)")
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("run aborted (stdin is not a tty)")
	}
	description := fmt.Sprintf(`The resource %s requires a run confirmation.
Please make sure you fully understand its role and effects before confirming the run.
Enter "yes" if you really want to run.`, t.RID())
	s, err := confirmation.ReadLn(description, time.Second*30)
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	if s == "yes" {
		t.Log().Infof("run confirmed interactively")
		return nil
	}
	return fmt.Errorf("run aborted")
}

// notifyRunDone is a noop here as for now the daemon api has no support for
// POST /run_done, and may not need one.
func (t T) notifyRunDone() error {
	return nil
}

func (t T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action:              "run",
		Option:              "schedule",
		Base:                "",
		RequireConfirmation: t.Confirmation,
		RequireProvisioned:  true,
		RequireCollector:    false,
	}
}
