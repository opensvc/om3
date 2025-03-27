package command

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

		pid           int
		commandString string
		stdout        []byte
		stderr        []byte
		started       bool // Prevent relaunch
		waited        bool // Prevent relaunch
		promptReader  *bufio.Reader
		wg            sync.WaitGroup

		ctx    context.Context
		cancel context.CancelFunc
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
	ErrPromptAbort    = errors.New("command: aborted by prompt")
)

func New(opts ...funcopt.O) *T {
	t := &T{
		ctx:             context.Background(),
		stdoutLogLevel:  zerolog.Disabled,
		stderrLogLevel:  zerolog.Disabled,
		logLevel:        zerolog.DebugLevel,
		commandLogLevel: zerolog.DebugLevel,
		okExitCodes:     []int{0},
	}
	_ = funcopt.Apply(t, opts...)
	if t.timeout > 0 {
		t.ctx, t.cancel = context.WithTimeout(t.ctx, t.timeout)
	}
	t.cmd = exec.CommandContext(t.ctx, t.name, t.args...)
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
	return t.stdout, nil
}

// Stdout returns stdout results of command (meaningful after Wait() or Run()),
// command created without funcopt WithBufferedStdout() return nil
// valid results
func (t T) Stdout() []byte {
	return t.stdout
}

// Stderr returns stderr results of command (meaningful after Wait() or Run())
// command created without funcopt WithBufferedStderr() return nil
func (t T) Stderr() []byte {
	return t.stderr
}

// Start prepare command, then call underlying cmd.Start()
// it takes care of preparing logging, timeout, stdout and stderr watchers
func (t *T) Start() (err error) {
	var readOut, readErr func()
	if t.started {
		return fmt.Errorf("%w", ErrAlreadyStarted)
	}
	var toCloseOnEarlyReturn []io.Closer
	if !t.prompt() {
		return ErrPromptAbort
	}
	if err = t.update(); err != nil {
		return err
	}

	defer func() {
		// close readers when cmd is not started
		if !t.started {
			for _, r := range toCloseOnEarlyReturn {
				_ = r.Close()
			}
		}
	}()

	parseLines := func(r io.Reader, onLine func(s string), b *[]byte) error {
		reader := bufio.NewReader(r)

		for {
			// Read until newline or EOF
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				if b != nil {
					*b = append(*b, line...)
				}
				if onLine != nil {
					onLine(string(bytes.TrimSuffix(line, []byte("\n"))))
				}
			}

			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return fmt.Errorf("read bytes: %v", err)
			}
		}
	}

	if t.stdoutLogLevel != zerolog.Disabled || t.bufferStdout || t.onStdoutLine != nil {
		var r io.ReadCloser
		if r, err = t.cmd.StdoutPipe(); err != nil {
			if t.log != nil {
				t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "command.Start() -> StdoutPipe(): %s", err)
			}
			return fmt.Errorf("%w", err)
		}
		toCloseOnEarlyReturn = append(toCloseOnEarlyReturn, r)

		onLine := func(s string) {
			if t.log != nil && t.stdoutLogLevel != zerolog.Disabled {
				t.log.Attr("out", s).Attr("pid", t.pid).Levelf(t.stdoutLogLevel, "stdout: "+s)
			}
			if t.onStdoutLine != nil {
				t.onStdoutLine(s)
			}
		}

		t.wg.Add(1)
		readOut = func() {
			defer t.wg.Done()
			if err := parseLines(r, onLine, &t.stdout); err != nil {
				if t.log != nil {
					t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "command parse stdout lines: %s", err)
				}
			}
			// explicit close call for situation where t.cmd.Wait() is not called
			_ = r.Close()
		}
	}

	if t.stderrLogLevel != zerolog.Disabled || t.bufferStderr || t.onStderrLine != nil {
		var r io.ReadCloser
		if r, err = t.cmd.StderrPipe(); err != nil {
			if t.log != nil {
				t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "command.Start() -> StderrPipe(): %s", err)
			}
			return fmt.Errorf("%w", err)
		}
		toCloseOnEarlyReturn = append(toCloseOnEarlyReturn, r)

		onLine := func(s string) {
			if t.log != nil && t.stderrLogLevel != zerolog.Disabled {
				if t.log != nil {
					t.log.Attr("err", s).Attr("pid", t.pid).Levelf(t.stderrLogLevel, "stderr: "+s)
				}
			}
			if t.onStderrLine != nil {
				t.onStderrLine(s)
			}
		}

		t.wg.Add(1)
		readErr = func() {
			defer t.wg.Done()
			if err := parseLines(r, onLine, &t.stderr); err != nil {
				if t.log != nil {
					t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "command parse stderr lines: %s", err)
				}
			}

			// explicit close call for situation where t.cmd.Wait() is not called
			_ = r.Close()
		}
	}

	if t.log != nil {
		if t.commandLogLevel != zerolog.Disabled && t.commandLogLevel > t.logLevel {
			t.log.Attr("cmd", t.cmd.String()).Levelf(t.commandLogLevel, "run %s", t.cmd)
		} else {
			t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "run %s", t.cmd)
		}
	}
	t.started = true
	if err = t.cmd.Start(); err != nil {
		if t.log != nil {
			t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "run %s: %s", t.cmd, err)
		}
		return fmt.Errorf("%w", err)
	}
	if t.cmd.Process != nil {
		t.pid = t.cmd.Process.Pid
	}
	if readOut != nil {
		go readOut()
	}
	if readErr != nil {
		go readErr()
	}
	return nil
}

func (t *T) Cmd() *exec.Cmd {
	return t.cmd
}

func (t *T) ExitCode() int {
	return t.cmd.ProcessState.ExitCode()
}

func (t *T) Wait() error {
	if t.waited {
		return ErrAlreadyWaited
	}
	t.waited = true
	if t.cancel != nil {
		defer t.cancel()
	}
	t.wg.Wait()
	err := t.cmd.Wait()
	if t.ctx.Err() == context.DeadlineExceeded {
		t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "wait exec: %s", err)
		return context.DeadlineExceeded
	}
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return t.checkExitCode(exitError.ExitCode())
		}
		if t.log != nil {
			t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "wait exec: %s", err)
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

func (t *T) prompt() bool {
	if t.promptReader == nil {
		return true
	}
	fmt.Println(t)
	for {
		fmt.Print("Do you want to proceed? (y/n): ")
		input, err := t.promptReader.ReadString('\n')
		if err != nil {
			fmt.Println("An error occurred while reading input. Please try again.", err)
			continue
		}

		// Trim newline and spaces, and convert to lowercase
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" || input == "yes" {
			fmt.Println("Proceeding...")
			return true
		} else if input == "n" || input == "no" {
			fmt.Println("Operation cancelled.")
			return false
		} else {
			fmt.Println("Invalid input. Please enter 'y' or 'n'.")
		}
	}
}
