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
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/anmitsu/go-shlex"
	"github.com/kballard/go-shellquote"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
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
		waitDelay    time.Duration
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
		stdoutWriter  *lineWriter
		stderrWriter  *lineWriter

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

// DefaultWaitDelay is how long a command is given, after it has exited, for
// the output it wrote to reach this process.
//
// It is the grace a well-behaved command never uses: what it wrote is already
// in the pipe when it exits, and the pipe reaches EOF as soon as the last
// holder of its write end is gone. The delay is for the command that hands a
// write end to something that outlives it, where EOF never comes at all.
const DefaultWaitDelay = 5 * time.Second

func New(opts ...funcopt.O) *T {
	t := &T{
		stdoutLogLevel:  zerolog.Disabled,
		stderrLogLevel:  zerolog.Disabled,
		logLevel:        zerolog.TraceLevel,
		commandLogLevel: zerolog.TraceLevel,
		okExitCodes:     []int{0},
		waitDelay:       DefaultWaitDelay,
	}
	_ = funcopt.Apply(t, opts...)
	if t.ctx == nil {
		t.ctx = context.Background()
	}
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
func (t *T) Output() ([]byte, error) {
	if err := t.Run(); err != nil {
		return []byte{}, err
	}
	return t.stdout, nil
}

// Stdout returns stdout results of command (meaningful after Wait() or Run()),
// command created without funcopt WithBufferedStdout() return nil
// valid results
func (t *T) Stdout() []byte {
	return t.stdout
}

// Stderr returns stderr results of command (meaningful after Wait() or Run())
// command created without funcopt WithBufferedStderr() return nil
func (t *T) Stderr() []byte {
	return t.stderr
}

// Start prepare command, then call underlying cmd.Start()
// it takes care of preparing logging, timeout, stdout and stderr watchers
func (t *T) Start() (err error) {
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

	if t.stdoutLogLevel != zerolog.Disabled || t.bufferStdout || t.onStdoutLine != nil {
		w := &lineWriter{
			onLine: func(s string) {
				if t.log != nil && t.stdoutLogLevel != zerolog.Disabled {
					t.log.Attr("out", s).Attr("pid", t.startedPID()).Levelf(t.stdoutLogLevel, "stdout: %s", s)
				}
				if t.onStdoutLine != nil {
					t.onStdoutLine(s)
				}
			},
		}
		if t.bufferStdout {
			w.collect = &t.stdout
		}
		t.stdoutWriter = w
		t.cmd.Stdout = w
	}

	if t.stderrLogLevel != zerolog.Disabled || t.bufferStderr || t.onStderrLine != nil {
		w := &lineWriter{
			onLine: func(s string) {
				if t.log != nil && t.stderrLogLevel != zerolog.Disabled {
					t.log.Attr("err", s).Attr("pid", t.startedPID()).Levelf(t.stderrLogLevel, "stderr: %s", s)
				}
				if t.onStderrLine != nil {
					t.onStderrLine(s)
				}
			},
		}
		if t.bufferStderr {
			w.collect = &t.stderr
		}
		t.stderrWriter = w
		t.cmd.Stderr = w
	}

	if t.log != nil {
		if t.commandLogLevel != zerolog.Disabled && t.commandLogLevel > t.logLevel {
			t.log.Attr("cmd", t.cmd.String()).Levelf(t.commandLogLevel, "run %s", t.cmd)
		} else {
			t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "run %s", t.cmd)
		}
	}
	// A command that leaves something behind holding its output pipes is a
	// command this would otherwise wait on for ever: the pipes reach EOF when
	// the last holder of their write end is gone, and a detaching command
	// hands those to a process that outlives it. The delay bounds that wait,
	// and starts counting only once the process has exited, so it costs a
	// well-behaved command nothing.
	t.cmd.WaitDelay = t.waitDelay

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
	return nil
}

// startedPID is the pid of the running command, for the goroutines copying
// its output.
//
// They are started by exec.Cmd.Start, which sets Process before it starts
// them, so what it holds is theirs to read. The pid field is not: it is
// written by whoever called Start, after Start has returned and after the
// command has begun writing.
func (t *T) startedPID() int {
	if t.cmd == nil || t.cmd.Process == nil {
		return 0
	}
	return t.cmd.Process.Pid
}

func (t *T) Cmd() *exec.Cmd {
	return t.cmd
}

func (t *T) ExitCode() int {
	return t.cmd.ProcessState.ExitCode()
}

// NormalizedExitCode returns the process exit status, and 128 + the signal
// number when the process was terminated by one, which is the shell
// convention and the value this package's own error and log entries already
// carry.
//
// ExitCode returns the raw os/exec value, which is -1 for a signaled process
// and says nothing about which signal. A caller comparing against a command's
// documented codes wants that one. A caller reporting the outcome to a person
// wants this one, and wants it to agree with the error text beside it.
//
// It is -1 when the process never ran, which is the one case no exit status
// exists for.
func (t *T) NormalizedExitCode() int {
	ps := t.cmd.ProcessState
	if ps == nil {
		return -1
	}
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok {
		if ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		if ws.Exited() {
			return ws.ExitStatus()
		}
	}
	return ps.ExitCode()
}

// flushWriters hands over what the command wrote after its last newline.
func (t *T) flushWriters() {
	if t.stdoutWriter != nil {
		_ = t.stdoutWriter.Close()
	}
	if t.stderrWriter != nil {
		_ = t.stderrWriter.Close()
	}
}

func (t *T) Wait() error {
	if t.waited {
		return ErrAlreadyWaited
	}
	t.waited = true
	if t.cancel != nil {
		defer t.cancel()
	}
	// The process is waited on first, and the readers after it. Waiting for
	// the readers first is waiting for EOF on pipes a detached grandchild
	// holds open, which never comes, and leaves the exited child unreaped:
	//
	//	4458 ?  Sl  /usr/bin/om <path> instance provision --leader
	//	4958 ?  Z    \_ [podman] <defunct>
	//
	// Wait closes the pipes when the delay expires, which is what lets the
	// readers finish at all.
	err := t.cmd.Wait()
	t.flushWriters()
	if errors.Is(err, exec.ErrWaitDelay) {
		// The process exited, and exited well: the delay expired on the
		// pipes alone. What the command was asked to do, it did, so it is
		// not failed for what it left behind, but the leftovers are worth
		// naming: they are why an orphan is holding a file descriptor.
		if t.log != nil {
			t.log.Attr("cmd", t.cmd.String()).Attr("pid", t.pid).Levelf(t.logLevel,
				"exited, and something it left behind held its output open longer than %s", t.waitDelay)
		}
		err = nil
	}
	if t.ctx.Err() == context.DeadlineExceeded {
		if t.log != nil {
			t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "wait exec: %s", err)
		}
		return context.DeadlineExceeded
	}
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return t.checkExitCode(normalizeExitCode(exitError))
		}
		if t.log != nil {
			t.log.Attr("cmd", t.cmd.String()).Levelf(t.logLevel, "wait exec: %s", err)
		}
		return err
	}
	return t.checkExitCode(t.ExitCode())
}

// normalizeExitCode returns a normalized process exit code from an *exec.ExitError
//
// If the process was terminated by a Unix signal, it returns 128 + signal number
// (shell convention, e.g. SIGKILL=9 -> 137).
// If the process exited normally, it returns the native exit status.
// Else, it returns exitError.ExitCode() from os/exec.
//
// This helps keep exit code handling consistent with common shell.
func normalizeExitCode(exitError *exec.ExitError) int {
	if ws, ok := exitError.Sys().(syscall.WaitStatus); ok {
		if ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		if ws.Exited() {
			return ws.ExitStatus()
		}
	}
	return exitError.ExitCode()
}

func (t *T) checkExitCode(exitCode int) error {
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

func (t *T) logExitCode(exitCode int) {
	if t.log != nil {
		t.log.Attr("cmd", t.cmd.String()).Attr("exit_code", exitCode).Levelf(t.logLevel, "pid %d exited with code %d", t.pid, exitCode)
	}
}

func (t *T) logErrorExitCode(exitCode int, err error) {
	if t.log != nil {
		t.log.Attr("cmd", t.cmd.String()).Attr("exit_code", exitCode).Levelf(t.errorExitCodeLogLevel, "pid %d exited with code %d", t.pid, exitCode)
	}
}

// serviceManagerEnv are the variables a service manager sets for the service
// it starts, and for that service alone.
//
// systemd tells a service where to answer it (NOTIFY_SOCKET), which
// descriptors it was handed (LISTEN_*) and which watchdog it has to feed
// (WATCHDOG_*). A command the service runs is not that service, and is not
// meant to read them: sd_notify has a flag to clear them for that reason, and
// this daemon cannot use it because it goes on notifying for as long as it
// runs.
//
// A container runtime finding NOTIFY_SOCKET waits for the container to report
// itself ready, and a container that is not a service never does: the
// container runs, "runc start" never returns, and the start that asked for it
// waits until its own timeout ends it.
var serviceManagerEnv = []string{
	"NOTIFY_SOCKET",
	"LISTEN_FDNAMES",
	"LISTEN_FDS",
	"LISTEN_PID",
	"WATCHDOG_PID",
	"WATCHDOG_USEC",
}

// withoutServiceManagerEnv is env without what a service manager addressed to
// this process alone.
func withoutServiceManagerEnv(env []string) []string {
	l := make([]string, 0, len(env))
	for _, s := range env {
		name, _, _ := strings.Cut(s, "=")
		if slices.Contains(serviceManagerEnv, name) {
			continue
		}
		l = append(l, s)
	}
	return l
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
	cmd.Env = withoutServiceManagerEnv(cmd.Env)
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
