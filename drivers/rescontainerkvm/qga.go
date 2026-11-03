package rescontainerkvm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/plog"
)

const (
	leakedPidsDir = "qemu-exec-leaked-pids"
)

// pidsCollected is used to ensure we only collect leaked pids once per om exec
var pidsCollected = false

type (
	qgaPingRequest struct {
		Execute string `json:"execute"`
	}
	qgaPingResponse struct {
		Return struct{} `json:"return"`
	}
	qgaCommandStatusRequest struct {
		Execute   string `json:"execute"`
		Arguments struct {
			PID int `json:"pid"`
		} `json:"arguments"`
	}
	qgaCommandStatus struct {
		Return struct {
			// ExitCode is the exit code of the program run in the container
			ExitCode int `json:"exitcode"`

			// OutData is in base64
			OutData string `json:"out-data"`

			// ErrData is in base64
			ErrData string `json:"err-data"`

			// Exited is true is the command run in the container is terminated
			Exited bool `json:"exited"`

			// Signal is the signal that ended the command, set in place
			// of the exit code
			Signal int `json:"signal"`
		} `json:"return"`
	}
	qgaCommandFileOpenRequest struct {
		Execute   string `json:"execute"`
		Arguments struct {
			Path string `json:"path"`
			Mode string `json:"mode"`
		} `json:"arguments"`
	}
	qgaCommandFileCloseRequest struct {
		Execute   string `json:"execute"`
		Arguments struct {
			Handle int `json:"handle"`
		} `json:"arguments"`
	}
	qgaCommandFileWriteRequest struct {
		Execute   string `json:"execute"`
		Arguments struct {
			Handle int    `json:"handle"`
			BufB64 string `json:"buf-b64"`
		} `json:"arguments"`
	}
	qgaCommandRequest struct {
		Execute   string `json:"execute"`
		Arguments struct {
			Path          string   `json:"path"`
			Arg           []string `json:"arg"`
			Env           []string `json:"env,omitempty"`
			InputData     string   `json:"input-data,omitempty"`
			CaptureOutput bool     `json:"capture-output"`
		} `json:"arguments"`
	}
	qgaExecCommandResponse struct {
		Return struct {
			PID int `json:"pid"`
		} `json:"return"`
	}
	qgaFileOpenCommandResponse struct {
		Return int `json:"return"`
	}
	qgaFileWriteCommandResponse struct {
		Return struct {
		} `json:"return"`
	}
	qgaFileCloseCommandResponse struct {
		Return struct {
		} `json:"return"`
	}

	qgaExecError struct {
		err      error
		exitCode int
	}

	qgaCommand struct {
		Ctx    context.Context
		Name   string
		Path   string
		Args   []string
		Envs   []string
		Stdin  io.Reader
		varDir string
		logger *plog.Logger

		// mu guards pid, status and started, which Signal reads from
		// another goroutine than the one running the command.
		mu      sync.Mutex
		pid     int
		status  *qgaCommandStatus
		started chan struct{}

		// startOver closes started once.
		startOver sync.Once
	}
)

// errQGACommandEnded says a command the guest agent ran has ended: there is
// nothing to signal.
var errQGACommandEnded = fmt.Errorf("the command has ended: %w", os.ErrProcessDone)

// errQGACommandNotStarted says a command the guest agent was to run did not
// start: there is nothing to signal.
var errQGACommandNotStarted = fmt.Errorf("the command did not start: %w", os.ErrProcessDone)

func (t *qgaExecError) Error() string {
	return fmt.Sprint(t.err)
}

func (t *qgaExecError) ExitCode() int {
	return t.exitCode
}

func (t *qgaCommand) Start() error {
	getInputData := func() (string, error) {
		if t.Stdin == nil {
			return "", nil
		}
		b, err := io.ReadAll(t.Stdin)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(b), nil
	}
	if t.status != nil {
		return nil
	}
	defer t.markStarted()
	// Try to free QTAILQ slots
	collectPids(t.Ctx, t.logger, t.varDir, t.Name)

	var response qgaExecCommandResponse
	request := qgaCommandRequest{
		Execute: "guest-exec",
	}
	request.Arguments.CaptureOutput = true
	request.Arguments.Path = t.Path
	request.Arguments.Arg = t.Args
	request.Arguments.Env = t.Envs
	if inputData, err := getInputData(); err != nil {
		return err
	} else {
		request.Arguments.InputData = inputData
	}
	err := qgaPost(t.Ctx, t.Name, &request, &response)
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.pid = response.Return.PID
	t.mu.Unlock()
	return nil
}

// startedChan returns the channel closed once the start is over, the pid
// known or the start failed.
func (t *qgaCommand) startedChan() chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.started == nil {
		t.started = make(chan struct{})
	}
	return t.started
}

// markStarted tells Signal the start is over.
func (t *qgaCommand) markStarted() {
	c := t.startedChan()
	t.startOver.Do(func() { close(c) })
}

func (t *qgaCommand) Run() error {
	err := t.Start()
	if err != nil {
		return err
	}
	return t.Wait()
}

func (t *qgaCommand) Wait() error {
	t.mu.Lock()
	ended := t.status
	t.mu.Unlock()
	if ended != nil {
		return statusError(ended)
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.Ctx.Done():
			t.mu.Lock()
			ended := t.status
			t.mu.Unlock()
			if ended != nil {
				// Read by Signal, which found it ended.
				return statusError(ended)
			}
			// Context was cancelled (likely timeout), write the pid for future cleanup
			writePid(t.Ctx, t.logger, t.varDir, t.Name, t.pid)
			return fmt.Errorf("timeout waiting for qemu guest exec result, pid %d", t.pid)
		case <-ticker.C:
			t.mu.Lock()
			if t.status != nil {
				// Read by Signal, which found it ended.
				ended := t.status
				t.mu.Unlock()
				return statusError(ended)
			}
			status, err := qgaExecStatus(t.Ctx, t.Name, t.pid)
			if err == nil && status.Return.Exited {
				t.status = status
			}
			t.mu.Unlock()
			if err != nil {
				return err
			}
			if status.Return.Exited {
				return statusError(status)
			}
		}
	}
}

// statusError returns the error of a command the guest agent reports ended:
// its exit code when not 0, 128 + the signal that ended it.
func statusError(status *qgaCommandStatus) error {
	if sig := status.Return.Signal; sig != 0 {
		// Ended by a signal: 128 + the signal, as the shell reports it.
		return &qgaExecError{exitCode: 128 + sig, err: fmt.Errorf("qga command ended by signal %d", sig)}
	}
	if status.Return.ExitCode != 0 {
		return &qgaExecError{exitCode: status.Return.ExitCode, err: fmt.Errorf("qga command exit code %d", status.Return.ExitCode)}
	}
	return nil
}

// Signal sends sig to the command started in the guest, by a kill the guest
// agent runs there: the agent has no operation stopping a command it started.
// The command then ends as the signal makes it, which Wait reports.
//
// A command still starting is signaled once started: Signal waits for the
// start to be over. A command ended, or which did not start, is not signaled,
// its pid may be another process's: an end is kept for Wait, and an error
// wrapping os.ErrProcessDone returned. The kill counts once it exited 0 in
// the guest.
func (t *qgaCommand) Signal(sig os.Signal) error {
	s, ok := sig.(syscall.Signal)
	if !ok {
		return fmt.Errorf("signal %v: not a system signal", sig)
	}
	select {
	case <-t.startedChan():
	case <-t.Ctx.Done():
		// Start fails on it: what started is tracked by Wait.
		return fmt.Errorf("signal %v: %w", sig, errQGACommandNotStarted)
	}
	t.mu.Lock()
	pid := t.pid
	t.mu.Unlock()
	if pid == 0 {
		return fmt.Errorf("signal %v: %w", sig, errQGACommandNotStarted)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if t.ended(ctx) {
		return errQGACommandEnded
	}
	if err := t.kill(ctx, s, pid); err != nil {
		if t.ended(ctx) {
			// Ended between the check and the kill.
			return errQGACommandEnded
		}
		return err
	}
	return nil
}

// ended tells if the agent reports the command ended, and keeps its end for
// Wait: the agent keeps the status of an ended command until read, so the
// pid is the command's own while it is not reported ended.
func (t *qgaCommand) ended(ctx context.Context) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status == nil {
		if status, err := qgaExecStatus(ctx, t.Name, t.pid); err == nil && status.Return.Exited {
			t.status = status
		}
	}
	return t.status != nil
}

// kill sends sig to pid by a kill the guest agent runs in the guest.
func (t *qgaCommand) kill(ctx context.Context, sig syscall.Signal, pid int) error {
	var response qgaExecCommandResponse
	request := qgaCommandRequest{
		Execute: "guest-exec",
	}
	request.Arguments.Path = "kill"
	request.Arguments.Arg = []string{fmt.Sprintf("-%d", int(sig)), strconv.Itoa(pid)}
	if err := qgaPost(ctx, t.Name, &request, &response); err != nil {
		return fmt.Errorf("signal %v: guest-exec kill: %w", sig, err)
	}
	// The kill runs asynchronously in the guest: it counts once it exited 0.
	killPID := response.Return.PID
	for {
		status, err := qgaExecStatus(ctx, t.Name, killPID)
		switch {
		case err != nil:
			writePid(ctx, t.logger, t.varDir, t.Name, killPID)
			return fmt.Errorf("signal %v: guest-exec kill status: %w", sig, err)
		case status.Return.Exited && status.Return.ExitCode != 0:
			return fmt.Errorf("signal %v: guest kill exit code %d", sig, status.Return.ExitCode)
		case status.Return.Exited:
			return nil
		}
		select {
		case <-ctx.Done():
			writePid(context.Background(), t.logger, t.varDir, t.Name, killPID)
			return fmt.Errorf("signal %v: guest kill not ended: %w", sig, ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (t *qgaCommand) StderrPipe() (io.ReadCloser, error) {
	if err := t.Run(); err != nil {
		return nil, err
	}
	b, err := base64.StdEncoding.DecodeString(t.status.Return.ErrData)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (t *qgaCommand) Output() ([]byte, error) {
	if err := t.Run(); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(t.status.Return.OutData)
}

func (t *qgaCommand) CombinedOutput() ([]byte, error) {
	if err := t.Run(); err != nil {
		return nil, err
	}
	outBytes, err := base64.StdEncoding.DecodeString(t.status.Return.OutData)
	if err != nil {
		return nil, err
	}
	errBytes, err := base64.StdEncoding.DecodeString(t.status.Return.ErrData)
	if err != nil {
		return nil, err
	}
	return append(outBytes, errBytes...), nil
}

func newQGAExecStatusCommandRequest(pid int) *qgaCommandStatusRequest {
	cmd := qgaCommandStatusRequest{
		Execute: "guest-exec-status",
	}
	cmd.Arguments.PID = pid
	return &cmd
}

func newQGAFileOpenCommand(path string, mode string) *qgaCommandFileOpenRequest {
	cmd := qgaCommandFileOpenRequest{
		Execute: "guest-file-open",
	}
	cmd.Arguments.Path = path
	cmd.Arguments.Mode = mode
	return &cmd
}

func newQGAFileWriteCommand(handle int, b []byte) *qgaCommandFileWriteRequest {
	cmd := qgaCommandFileWriteRequest{
		Execute: "guest-file-write",
	}
	cmd.Arguments.Handle = handle
	cmd.Arguments.BufB64 = base64.StdEncoding.EncodeToString(b)
	return &cmd
}

func newQGAFileCloseCommand(handle int) *qgaCommandFileCloseRequest {
	cmd := qgaCommandFileCloseRequest{
		Execute: "guest-file-close",
	}
	cmd.Arguments.Handle = handle
	return &cmd
}

func qgaPost(ctx context.Context, name string, request any, result any) error {
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return err
	}
	args := []string{"qemu-agent-command", name, string(requestBytes)}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("virsh"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
	)
	err = cmd.Run()
	//fmt.Println(">>>", cmd.Args)
	//fmt.Println("<<<", string(b), err)
	if err != nil {
		return fmt.Errorf("%w: %s", err, cmd.Stderr())
	}
	if err := json.Unmarshal(cmd.Stdout(), result); err != nil {
		return err
	}
	return nil
}
func qgaFileOpen(ctx context.Context, name, path, mode string) (int, error) {
	var response qgaFileOpenCommandResponse
	request := newQGAFileOpenCommand(path, mode)
	err := qgaPost(ctx, name, request, &response)
	if err != nil {
		return 0, err
	}
	return response.Return, nil
}

func qgaFileClose(ctx context.Context, name string, handle int) error {
	var response qgaFileCloseCommandResponse
	request := newQGAFileCloseCommand(handle)
	return qgaPost(ctx, name, request, &response)
}

func qgaFileWrite(ctx context.Context, name string, handle int, b []byte) error {
	var response qgaFileWriteCommandResponse
	request := newQGAFileWriteCommand(handle, b)
	err := qgaPost(ctx, name, request, &response)
	if err != nil {
		return err
	}
	return nil
}

func qgaCp(ctx context.Context, name, src, dst string) error {
	handle, err := qgaFileOpen(ctx, name, dst, "w")
	if err != nil {
		return err
	}
	defer qgaFileClose(ctx, name, handle)
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := qgaFileWrite(ctx, name, handle, b); err != nil {
		return err
	}
	return nil
}

func qgaExecStatus(ctx context.Context, name string, pid int) (*qgaCommandStatus, error) {
	var response qgaCommandStatus
	request := newQGAExecStatusCommandRequest(pid)
	err := qgaPost(ctx, name, request, &response)
	if err != nil {
		return nil, err
	}
	//fmt.Printf("%s [%d]: qga response: %#v\n", name, pid, response)
	return &response, nil
}

func qgaPing(ctx context.Context, name string) error {
	var response qgaPingResponse
	request := qgaPingRequest{
		Execute: "guest-ping",
	}
	return qgaPost(ctx, name, request, &response)
}

// pidFile returns the path to the pid file for tracking leaked qemu exec pids
func pidFile(varDir string, pid int) string {
	return filepath.Join(varDir, leakedPidsDir, fmt.Sprintf("%d", pid))
}

// collectPids tries to free QTAILQ slots by checking on previously leaked pids
func collectPids(ctx context.Context, logger *plog.Logger, varDir, name string) {
	if pidsCollected {
		return
	}
	pidsCollected = true

	pidDir := filepath.Join(varDir, leakedPidsDir)
	entries, err := os.ReadDir(pidDir)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Debugf("collectPids: read dir %s: %s", pidDir, err)
		}
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		pidStr := entry.Name()
		var pid int
		if _, err := fmt.Sscanf(pidStr, "%d", &pid); err != nil {
			logger.Debugf("collectPids: invalid pid filename %s: %s", pidStr, err)
			continue
		}

		pidPath := pidFile(varDir, pid)
		status, err := qgaExecStatus(ctx, name, pid)
		if err != nil {
			logger.Debugf("collectPids: exec_status for pid %d", pid, err)
			if removeErr := os.Remove(pidPath); removeErr != nil {
				logger.Debugf("collectPids: remove %s: %s", pidPath, removeErr)
			}
			continue
		}

		if status.Return.Exited {
			logger.Debugf("collectPids: remove %s: exec exited", pidPath)
			if removeErr := os.Remove(pidPath); removeErr != nil {
				logger.Debugf("collectPids: remove %s: %s", pidPath, removeErr)
			}
		} else {
			logger.Debugf("collectPids: keep %s: exec still not exited", pidPath)
		}
	}
}

// writePid writes a pid file to track a leaked qemu exec pid for future cleanup
func writePid(ctx context.Context, logger *plog.Logger, varDir, name string, pid int) {
	pidPath := pidFile(varDir, pid)
	logger.Debugf("write %s for a future 'guest-exec-status <pid>'", pidPath)

	// Ensure the directory exists
	pidDir := filepath.Join(varDir, leakedPidsDir)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		logger.Debugf("writePid: mkdir %s", pidDir, err)
		return
	}

	f, err := os.Create(pidPath)
	if err != nil {
		logger.Debugf("writePid: create %s", pidPath, err)
		return
	}
	defer f.Close()
	// Empty file, just for tracking
}
