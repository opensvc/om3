package command

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/anmitsu/go-shlex"
	"github.com/kballard/go-shellquote"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
)

type (
	T struct {
		name         string
		args         []string
		bufferStdout bool
		bufferStderr bool
		user         string
		group        string
		cwd          string
		env          []string
		cmd          *exec.Cmd
		label        string
		timeout      time.Duration
		onStdoutLine func(string)
		onStderrLine func(string)
		okExitCodes  []int

		log                   *plog.Logger
		logLevel              zerolog.Level
		commandLogLevel       zerolog.Level
		errorExitCodeLogLevel zerolog.Level
		stdoutLogLevel        zerolog.Level
		stderrLogLevel        zerolog.Level

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
		name         string
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

// Output returns stdout results of command (meaningful after Wait() or Run()),
// command created without funcopt WithBufferedStdout() return nil
// valid results
func (t T) Output() ([]byte, error) {
	if err := t.Run(); err != nil {
		return []byte{}, err
	}
	return stripFistByte(t.stdout), nil
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
		return fmt.Errorf("%w", ErrAlreadyStarted)
	}
	t.started = true
	cmd := t.Cmd()
	if err = t.update(); err != nil {
		return err
	}
	if t.stdoutLogLevel != zerolog.Disabled || t.bufferStdout || t.onStdoutLine != nil {
		var r io.ReadCloser
		if r, err = cmd.StdoutPipe(); err != nil {
			if t.log != nil {
				t.log.Attr("cmd", cmd.String()).Levelf(t.logLevel, "command.Start() -> StdoutPipe(): %s", err)
			}
			return fmt.Errorf("%w", err)
		}
		t.closeAfterStart = append(t.closeAfterStart, r)
		t.goroutine = append(t.goroutine, func() {
			s := bufio.NewScanner(r)
			for s.Scan() {
				if t.log != nil && t.stdoutLogLevel != zerolog.Disabled {
					t.log.Attr("out", s.Text()).Attr("pid", t.pid).Levelf(t.stdoutLogLevel, "stdout: "+s.Text())
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
			if t.log != nil {
				t.log.Attr("cmd", cmd.String()).Levelf(t.logLevel, "command.Start() -> StderrPipe(): %s", err)
			}
			return fmt.Errorf("%w", err)
		}
		t.closeAfterStart = append(t.closeAfterStart, r)
		t.goroutine = append(t.goroutine, func() {
			s := bufio.NewScanner(r)
			for s.Scan() {
				if t.log != nil && t.stderrLogLevel != zerolog.Disabled {
					t.log.Attr("err", s.Text()).Attr("pid", t.pid).Levelf(t.stdoutLogLevel, "stderr: "+s.Text())
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
		if t.log != nil {
			t.log.Levelf(t.logLevel, "use context %v", ctx)
		}
		t.goroutine = append(t.goroutine, func() {
			select {
			case <-ctx.Done():
				err := ctx.Err()
				if err == context.DeadlineExceeded {
					if cmd.Process == nil {
						if t.log != nil {
							t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "deadlineExceeded, but cmd.Process is nil")
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
						t.log.Attr("pid", t.pid).Levelf(t.stderrLogLevel, "deadlineExceeded, pid is %d", t.pid)
					} else if t.log != nil {
						t.log.Attr("pid", t.pid).Levelf(t.logLevel, "deadlineExceeded, pid is %d", t.pid)
					}
					if t.log != nil {
						t.log.Attr("cmd", t.cmd.String()).Attr("pid", t.pid).Levelf(t.logLevel, "kill deadline exceeded pid %d", t.pid)
					}
					err := cmd.Process.Kill()
					if err != nil && t.log != nil {
						t.log.Attr("cmd", t.cmd.String()).Attr("pid", t.pid).Levelf(t.logLevel, "kill deadline exceeded pid %d: %s", t.pid, err)
					}
				}
			}
			// don't need to wait on other go routines
			for i := 0; i < len(t.goroutine); i++ {
				t.done <- "ctx"
			}
		})
	}
	if t.log != nil {
		if t.commandLogLevel != zerolog.Disabled && t.commandLogLevel > t.logLevel {
			t.log.Attr("cmd", t.cmd.String()).Levelf(t.commandLogLevel, "run %s", t.cmd)
		} else {
			t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "run %s", t.cmd)
		}
	}
	if err = cmd.Start(); err != nil {
		if t.log != nil {
			t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "run %s: %s", t.cmd, err)
		}
		return fmt.Errorf("%w", err)
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
	if t.cmd == nil {
		t.cmd = exec.Command(t.name, t.args...)
	}
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
	// wait for of goroutines
	for i := 0; i < waitCount; i++ {
		if t.log != nil {
			t.log.Levelf(t.logLevel, "end of goroutine %v", <-t.done)
		} else {
			<-t.done
		}
	}
	cmd := t.cmd
	if err := cmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return t.checkExitCode(exitError.ExitCode())
		}
		if t.log != nil {
			t.log.Attr("cmd", cmd.String()).Levelf(t.logLevel, "cmd.Wait(): %s", err)
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
	err := &ErrExitCode{name: t.name, exitCode: exitCode, successCodes: t.okExitCodes}
	t.logErrorExitCode(exitCode, err)
	return fmt.Errorf("%w", err)
}

func (e *ErrExitCode) ExitCode() int {
	return e.exitCode
}

func (e *ErrExitCode) Error() string {
	return fmt.Sprintf("%s exit code %v not in success codes: %v", e.name, e.exitCode, e.successCodes)
}

func (t T) logExitCode(exitCode int) {
	if t.log != nil {
		t.log.Attr("cmd", t.cmd.String()).Attr("exit_code", exitCode).Levelf(t.logLevel, "pid %d exited with code %d", t.pid, exitCode)
	}
}

func (t T) logErrorExitCode(exitCode int, err error) {
	if t.log != nil {
		t.log.Attr("cmd", t.cmd.String()).Attr("exit_code", exitCode).Levelf(t.errorExitCodeLogLevel, "pid %d exited with code %d", t.pid, exitCode)
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
	cmd.Env = os.Environ()
	if len(t.env) > 0 {
		cmd.Env = append(cmd.Env, t.env...)
	}
	if credential, err := credential(t.user, t.group); err != nil {
		if t.log != nil {
			t.log.Levelf(t.logLevel, "unable to set credential from user '%v', group '%v' for action '%v': %s", t.user, t.group, t.label, err)
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
		return nil, fmt.Errorf("can not create command from empty string")
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
		return nil, fmt.Errorf("unexpected empty command args from string")
	}
	return sSplit, nil
}

// CmdArgsFromString returns args for exec.Command from a string command 's'
// When string command 's' contains multiple commands,
//
//	exec.Command("/bin/sh", "-c", s)
//
// else
//
//	exec.Command from shlex.Split(s)
func CmdArgsFromString(s string) ([]string, error) {
	return commandArgsFromString(s)
}

func (t *T) toString() string {
	if t.name == "" {
		return ""
	}
	fp, _ := exec.LookPath(t.name)
	fp, _ = filepath.Abs(fp)
	argv := append([]string{fp}, t.args...)
	return shellquote.Join(argv...)
}

func stripFistByte(b []byte) []byte {
	if len(b) > 1 {
		return b[1:]
	}
	return b
}
