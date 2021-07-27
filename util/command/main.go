package command

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/anmitsu/go-shlex"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		name            string
		args            []string
		log             *zerolog.Logger
		logLevel        zerolog.Level
		commandLogLevel zerolog.Level
		stdoutLogLevel  zerolog.Level
		stderrLogLevel  zerolog.Level
		bufferStdout    bool
		bufferStderr    bool
		user            string
		group           string
		cwd             string
		env             []string
		cmd             *exec.Cmd
		label           string
		timeout         time.Duration
		onStdoutLine    func(string)
		onStderrLine    func(string)
		okExitCodes     []int

		pid             int
		commandString   string
		done            chan string
		goroutine       []func()
		cancel          func()
		ctx             context.Context
		closeAfterStart []io.Closer
		stdout          []byte
		stderr          []byte
		started         bool // Prevent relaunch
		waited          bool // Prevent relaunch
	}

	ErrExitCode struct {
		exitCode     int
		successCodes []int
	}
)

var (
	ErrAlreadyStarted = errors.New("command: already started")
	ErrAlreadyWaited  = errors.New("command: already waited")
)

func New(opts ...funcopt.O) *T {
	t := &T{
		stdoutLogLevel:  zerolog.Disabled,
		stderrLogLevel:  zerolog.Disabled,
		logLevel:        zerolog.DebugLevel,
		commandLogLevel: zerolog.DebugLevel,
		okExitCodes:     []int{0},
	}
	_ = funcopt.Apply(t, opts...)
	return t
}

func (t *T) String() string {
	if len(t.commandString) != 0 {
		return t.commandString
	}
	t.commandString = t.toString()
	return t.commandString
}

func (t *T) Run() error {
	if err := t.Start(); err != nil {
		return err
	}
	return t.Wait()
}

// Stdout returns stdout results of command (meaningful after Wait() or Run()),
// command created without funcopt WithBufferedStdout() return nil
// valid results
func (t T) Stdout() []byte {
	return stripFistByte(t.stdout)
}

// Stderr returns stderr results of command (meaningful after Wait() or Run())
// command created without funcopt WithBufferedStderr() return nil
func (t T) Stderr() []byte {
	return stripFistByte(t.stderr)
}

// Start prepare command, then call underlying cmd.Start()
// it takes care of preparing logging, timeout, stdout and stderr watchers
func (t *T) Start() (err error) {
	if t.started {
		return ErrAlreadyStarted
	}
	t.started = true
	cmd := exec.Command(t.name, t.args...)
	t.cmd = cmd
	if err = t.update(); err != nil {
		return err
	}
	log := t.log
	if t.stdoutLogLevel != zerolog.Disabled || t.bufferStdout || t.onStdoutLine != nil {
		var r io.ReadCloser
		if r, err = cmd.StdoutPipe(); err != nil {
			if log != nil {
				log.WithLevel(t.logLevel).Err(err).Str("cmd", cmd.String()).Msg("command.Start() -> StdoutPipe()")
			}
			return err
		}
		t.closeAfterStart = append(t.closeAfterStart, r)
		t.goroutine = append(t.goroutine, func() {
			s := bufio.NewScanner(r)
			for s.Scan() {
				if log != nil && t.stdoutLogLevel != zerolog.Disabled {
					log.WithLevel(t.stdoutLogLevel).Str("out", s.Text()).Int("pid", t.pid).Send()
				}
				if t.onStdoutLine != nil {
					t.onStdoutLine(s.Text())
				}
				if t.bufferStdout {
					t.stdout = append(t.stdout, append([]byte("\n"), s.Bytes()...)...)
				}
			}
			t.done <- "stdout"
		})
	}
	if t.stderrLogLevel != zerolog.Disabled || t.bufferStderr || t.onStderrLine != nil {
		var r io.ReadCloser
		if r, err = cmd.StderrPipe(); err != nil {
			if log != nil {
				log.WithLevel(t.logLevel).Err(err).Str("cmd", cmd.String()).Msg("command.Start() -> StderrPipe()")
			}
			return err
		}
		t.closeAfterStart = append(t.closeAfterStart, r)
		t.goroutine = append(t.goroutine, func() {
			s := bufio.NewScanner(r)
			for s.Scan() {
				if log != nil && t.stderrLogLevel != zerolog.Disabled {
					log.WithLevel(t.stderrLogLevel).Str("err", s.Text()).Int("pid", t.pid).Send()
				}
				if t.onStderrLine != nil {
					t.onStderrLine(s.Text())
				}
				if t.bufferStderr {
					t.stderr = append(t.stderr, append([]byte("\n"), s.Bytes()...)...)
				}
			}
			t.done <- "stderr"
		})
	}
	if t.timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
		t.ctx = ctx
		t.cancel = cancel
		if log != nil {
			log.WithLevel(t.logLevel).Msgf("use context %v", ctx)
		}
		t.goroutine = append(t.goroutine, func() {
			select {
			case <-ctx.Done():
				err := ctx.Err()
				if err == context.DeadlineExceeded {
					if cmd.Process == nil {
						if log != nil {
							log.WithLevel(t.logLevel).Err(err).Str("cmd", t.cmd.String()).Msg("DeadlineExceeded, but cmd.Process is nil")
						}
						// don't need to wait on other go routines
						for i := 0; i < len(t.goroutine); i++ {
							t.done <- "ctx"
						}
						return
					}
					if t.onStderrLine != nil {
						t.onStderrLine("DeadlineExceeded")
					}
					if t.stderrLogLevel != zerolog.Disabled {
						log.WithLevel(t.stderrLogLevel).Str("err", "DeadlineExceeded").Int("pid", t.pid).Send()
					} else if t.log != nil {
						log.WithLevel(t.logLevel).Str("err", "DeadlineExceeded").Int("pid", t.pid).Send()
					}
					if log != nil {
						log.WithLevel(t.logLevel).Err(err).Str("cmd", t.cmd.String()).Int("pid", t.pid).Msg("kill DeadlineExceeded pid")
					}
					err := cmd.Process.Kill()
					if err != nil && log != nil {
						log.WithLevel(t.logLevel).Err(err).Str("cmd", t.cmd.String()).Int("pid", t.pid).Msg("kill DeadlineExceeded pid failed")
					}
				}
			}
			// don't need to wait on other go routines
			for i := 0; i < len(t.goroutine); i++ {
				t.done <- "ctx"
			}
		})
	}
	if log != nil && t.commandLogLevel != zerolog.Disabled {
		log.WithLevel(t.commandLogLevel).Str("cmd", cmd.String()).Msg("running")
	}
	if log != nil {
		log.WithLevel(t.logLevel).Str("cmd", cmd.String()).Msg("running")
	}
	if err = cmd.Start(); err != nil {
		if log != nil {
			log.WithLevel(t.logLevel).Err(err).Str("cmd", cmd.String()).Msg("running")
		}
		return err
	}
	if cmd.Process != nil {
		t.pid = cmd.Process.Pid
	}
	if len(t.goroutine) > 0 {
		t.done = make(chan string, len(t.goroutine))
		for _, f := range t.goroutine {
			go f()
		}
	}
	return nil
}

func (t *T) Cmd() *exec.Cmd {
	return t.cmd
}

func (t *T) ExitCode() int {
	return t.cmd.ProcessState.ExitCode()
}

func (t *T) Wait() (err error) {
	if t.waited {
		return ErrAlreadyWaited
	}
	t.waited = true
	waitCount := len(t.goroutine)
	if t.cancel != nil {
		waitCount = waitCount - 1
		defer t.cancel()
	}
	log := t.log
	// wait for of goroutines
	for i := 0; i < waitCount; i++ {
		if log != nil {
			log.WithLevel(t.logLevel).Msgf("end of goroutine %v", <-t.done)
		} else {
			<-t.done
		}
	}
	cmd := t.cmd
	if err := cmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return t.checkExitCode(exitError.ExitCode())
		}
		if log != nil {
			log.WithLevel(t.logLevel).Err(err).Str("cmd", cmd.String()).Msg("cmd.Wait()")
		}
		return err
	}
	return t.checkExitCode(t.ExitCode())
}

func (t T) checkExitCode(exitCode int) error {
	if len(t.okExitCodes) == 0 {
		t.logExitCode(exitCode)
		return nil
	}
	for _, validCode := range t.okExitCodes {
		if exitCode == validCode {
			t.logExitCode(exitCode)
			return nil
		}
	}
	err := &ErrExitCode{exitCode: exitCode, successCodes: t.okExitCodes}
	t.logErrorExitCode(exitCode, err)
	return err
}

func (e *ErrExitCode) Error() string {
	return fmt.Sprintf("command exit code %v not in success codes: %v", e.exitCode, e.successCodes)
}

func (t T) logExitCode(exitCode int) {
	if t.log != nil {
		t.log.WithLevel(t.logLevel).Str("cmd", t.cmd.String()).Int("exitCode", exitCode).Send()
	}
}

func (t T) logErrorExitCode(exitCode int, err error) {
	if t.log != nil {
		t.log.WithLevel(t.logLevel).Err(err).Str("cmd", t.cmd.String()).Int("exitCode", exitCode).Send()
	}
}

// Update t.cmd with options
func (t *T) update() error {
	cmd := t.cmd
	if cmd == nil {
		panic("command.update() called with cmd nil")
	}
	if t.cwd != "" {
		cmd.Dir = t.cwd
	}
	if len(t.env) > 0 {
		cmd.Env = append(cmd.Env, t.env...)
	}
	if credential, err := credential(t.user, t.group); err != nil {
		if t.log != nil {
			t.log.WithLevel(t.logLevel).Err(err).Msgf("unable to set credential from user '%v', group '%v' for action '%v'", t.user, t.group, t.label)
		}
		return err
	} else if credential != nil {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Credential = credential
	}
	t.commandString = t.toString()
	return nil
}

func commandArgsFromString(s string) ([]string, error) {
	var needShell bool
	if len(s) == 0 {
		return nil, errors.New("can not create command from empty string")
	}
	switch {
	case strings.Contains(s, "|"):
		needShell = true
	case strings.Contains(s, "&&"):
		needShell = true
	case strings.Contains(s, ";"):
		needShell = true
	}
	if needShell {
		return []string{"/bin/sh", "-c", s}, nil
	}
	sSplit, err := shlex.Split(s, true)
	if err != nil {
		return nil, err
	}
	if len(sSplit) == 0 {
		return nil, errors.New("unexpected empty command args from string")
	}
	return sSplit, nil
}

// CmdArgsFromString returns args for exec.Command from a string command 's'
// When string command 's' contains multiple commands,
//   exec.Command("/bin/sh", "-c", s)
// else
//   exec.Command from shlex.Split(s)
func CmdArgsFromString(s string) ([]string, error) {
	return commandArgsFromString(s)
}

func (t *T) toString() string {
	if len(t.args) == 0 {
		return t.name
	}
	var args []string
	for _, arg := range t.args {
		args = append(args, fmt.Sprintf("%q", arg))
	}
	return fmt.Sprintf("%v %s", t.name, strings.Join(args, " "))
}

func stripFistByte(b []byte) []byte {
	if len(b) > 1 {
		return b[1:]
	}
	return b
}
