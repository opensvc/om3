package restaskhost

// TODO
// * snooze
// * (R)unning status flag
// * notify_done

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/atomicgo/cursor"
	"github.com/eiannone/keyboard"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/object"
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
	RunCmd       string
	OnErrorCmd   string
	Check        string
	Confirmation bool
	LogOutputs   bool
	Snooze       *time.Duration
}

func New() resource.Driver {
	return &T{}
}

func init() {
	resource.Register(driverGroup, driverName, New)
}

func (t T) IsRunning() bool {
	err := t.DoWithLock(false, time.Second*0, "run", func() error {
		return nil
	})
	return err != nil
}

// Start the Resource
func (t T) Start(ctx context.Context) (err error) {
	return nil
}

func (t T) Run(ctx context.Context) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	err := t.DoWithLock(disable, timeout, "run", func() error {
		return t.lockedRun(ctx)
	})
	return err
}

func (t T) lockedRun(ctx context.Context) (err error) {
	var opts []funcopt.O
	if err := t.handleConfirmation(ctx); err != nil {
		return err
	}
	if opts, err = t.GetFuncOpts(t.RunCmd, "run"); err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}
	if err := t.ApplyPGChain(ctx); err != nil {
		return err
	}
	if t.LogOutputs {
		opts = append(opts,
			command.WithLogger(t.Log()),
			command.WithStdoutLogLevel(zerolog.InfoLevel),
			command.WithStderrLogLevel(zerolog.WarnLevel),
		)
	}
	opts = append(opts,
		command.WithTimeout(t.GetTimeout("run")),
		command.WithIgnoredExitCodes(),
	)
	cmd := command.New(opts...)
	t.Log().Info().Stringer("cmd", cmd).Msg("run")
	err = cmd.Run()
	if err := t.writeLastRun(cmd.ExitCode()); err != nil {
		return err
	}
	if err != nil {
		t.Log().Err(err).Msg("")
		if err := t.onError(); err != nil {
			t.Log().Warn().Msgf("%s", err)
		}
	}
	if s, err := t.ExitCodeToStatus(cmd.ExitCode()); err != nil {
		return err
	} else if s != status.Up {
		return fmt.Errorf("command exited with code %d", cmd.ExitCode())
	}
	return object.ErrLogged
}

func (t T) onError() error {
	opts, err := t.GetFuncOpts(t.OnErrorCmd, "on_error")
	if err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}
	cmd := command.New(opts...)
	t.Log().Info().Stringer("cmd", cmd).Msg("on error run")
	return cmd.Run()
}

func (t *T) Stop(ctx context.Context) error {
	return nil
}

func (t *T) Kill(ctx context.Context) error {
	if t.StopCmd != "" {
		return t.CommonStop(ctx, t)
	}
	return t.stop(ctx)
}

func (t *T) stop(ctx context.Context) error {
	cmdArgs, err := t.BaseCmdArgs(t.StartCmd, "stop")
	if err != nil {
		return err
	}
	procs, err := t.getRunning(cmdArgs)
	if err != nil {
		return err
	}
	if procs.Len() == 0 {
		t.Log().Info().Msg("already stopped")
		return nil
	}
	for _, p := range procs.Procs() {
		t.Log().Info().Str("cmd", p.CommandLine()).Msgf("send termination signal to process %d", p.PID())
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
				t.Log().Info().Str("cmd", p.CommandLine()).Msgf("process %d is now terminated", p.PID())
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
	if b, err := ioutil.ReadFile(p); err != nil {
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
	var s status.T
	if t.CheckCmd != "" {
		s = t.CommonStatus(ctx)
	} else {
		s = t.status()
	}
	return s == status.Up
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return ""
}

func (t *T) status() status.T {
	cmdArgs, err := t.BaseCmdArgs(t.StartCmd, "start")
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	procs, err := t.getRunning(cmdArgs)
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

func (t T) getRunning(cmdArgs []string) (proc.L, error) {
	procs, err := proc.All()
	if err != nil {
		return procs, err
	}
	procs = procs.FilterByEnv("OPENSVC_ID", t.ObjectID.String())
	procs = procs.FilterByEnv("OPENSVC_RID", t.RID())
	return procs, nil
}

func (t T) handleConfirmation(ctx context.Context) error {
	if !t.Confirmation {
		return nil
	}
	if actioncontext.IsConfirm(ctx) {
		t.Log().Info().Msg("run confirmed by command line option")
		return nil
	}
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return nil
	}
	fmt.Printf(`The resource %s requires a run confirmation.
Please make sure you fully understand its role and effects before confirming the run.
Enter "yes" if you really want to run.`, t.RID())
	fmt.Println("")
	s, err := Readln(time.Second * 30)
	if err != nil {
		return errors.Wrap(err, "read confirmation")
	}
	if s == "yes" {
		t.Log().Info().Msg("run confirmed interactively")
		return nil
	}
	return fmt.Errorf("run aborted")
}

func Readln(timeout time.Duration) (string, error) {
	keysEvents, err := keyboard.GetKeys(10)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = keyboard.Close()
	}()

	area := cursor.NewArea()
	end := time.Now().Add(timeout)
	word := ""
	pos := 0

	prompt := func() bool {
		left := end.Sub(time.Now())
		if left < 0 {
			return false
		}
		area.Clear()
		s := fmt.Sprintf("Timeout in %s > %s", left.Round(time.Second), word)
		area.Update(s)
		cursor.Up(1)
		cursor.HorizontalAbsolute(len(s) - pos)
		return true
	}

	_ = prompt()

	for {
		select {
		case ev := <-keysEvents:
			switch {
			case ev.Err != nil:
				fmt.Println("")
				return "", err
			case (ev.Key == keyboard.KeyBackspace) || (ev.Key == keyboard.KeyBackspace2):
				if len(word) > 0 {
					word = word[0 : len(word)-1]
				}
				_ = prompt()
			case ev.Key == keyboard.KeyCtrlC:
				fmt.Println("")
				return "", fmt.Errorf("interrupted")
			case ev.Key == keyboard.KeyEnter:
				fmt.Println("")
				return word, nil
			case ev.Key == keyboard.KeyArrowLeft:
				if pos < len(word) {
					pos = pos + 1
					_ = prompt()
				}
			case ev.Key == keyboard.KeyArrowRight:
				if pos > 0 {
					pos = pos - 1
					_ = prompt()
				}
			case ev.Key == keyboard.KeyCtrlE:
				if pos > 0 {
					pos = 0
					_ = prompt()
				}
			case ev.Key == keyboard.KeyCtrlA:
				if pos < len(word) {
					pos = len(word)
					_ = prompt()
				}
			case ev.Key == keyboard.KeyCtrlK:
				if pos > 0 {
					word = word[0 : len(word)-pos]
					pos = 0
					_ = prompt()
				}
			case ev.Key == keyboard.KeySpace:
				offset := len(word) - pos
				word = word[0:offset] + " " + word[offset:len(word)]
				_ = prompt()
			case ev.Rune != '0':
				offset := len(word) - pos
				word = word[0:offset] + string(ev.Rune) + word[offset:len(word)]
				_ = prompt()
			}
		case <-time.After(time.Second):
			if more := prompt(); !more {
				fmt.Println("")
				return "", fmt.Errorf("timeout")
			}
		}
	}
	fmt.Println("")
	return "", nil
}
