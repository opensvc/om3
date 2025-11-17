package rescontainerkvm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/opensvc/om3/util/command"
)

type (
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
			Env           []string `json:"env"`
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
		Ctx   context.Context
		Name  string
		Path  string
		Args  []string
		Envs  []string
		Stdin io.Reader

		pid    int
		status *qgaCommandStatus
	}
)

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
	err := qgaPost(t.Name, &request, &response)
	if err != nil {
		return err
	}
	t.pid = response.Return.PID
	return nil
}

func (t *qgaCommand) Run() error {
	err := t.Start()
	if err != nil {
		return err
	}
	return t.Wait()
}

func (t *qgaCommand) Wait() error {
	if t.status != nil {
		return nil
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.Ctx.Done():
			return nil
		case <-ticker.C:
			status, err := qgaExecStatus(t.Name, t.pid)
			if err != nil {
				return err
			}
			if status.Return.Exited {
				t.status = status
				if status.Return.ExitCode != 0 {
					return &qgaExecError{exitCode: status.Return.ExitCode, err: fmt.Errorf("qga command exit code %d", status.Return.ExitCode)}
				}
				return nil
			}
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

func qgaPost(name string, request any, result any) error {
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return err
	}
	args := []string{"qemu-agent-command", name, string(requestBytes)}
	cmd := command.New(
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
func qgaFileOpen(name, path, mode string) (int, error) {
	var response qgaFileOpenCommandResponse
	request := newQGAFileOpenCommand(path, mode)
	err := qgaPost(name, request, &response)
	if err != nil {
		return 0, err
	}
	return response.Return, nil
}

func qgaFileClose(name string, handle int) error {
	var response qgaFileCloseCommandResponse
	request := newQGAFileCloseCommand(handle)
	return qgaPost(name, request, &response)
}

func qgaFileWrite(name string, handle int, b []byte) error {
	var response qgaFileWriteCommandResponse
	request := newQGAFileWriteCommand(handle, b)
	err := qgaPost(name, request, &response)
	if err != nil {
		return err
	}
	return nil
}

func qgaCp(ctx context.Context, name, src, dst string) error {
	handle, err := qgaFileOpen(name, dst, "w")
	if err != nil {
		return err
	}
	defer qgaFileClose(name, handle)
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := qgaFileWrite(name, handle, b); err != nil {
		return err
	}
	return nil
}

func qgaExecStatus(name string, pid int) (*qgaCommandStatus, error) {
	var response qgaCommandStatus
	request := newQGAExecStatusCommandRequest(pid)
	err := qgaPost(name, request, &response)
	if err != nil {
		return nil, err
	}
	//fmt.Printf("%s [%d]: qga response: %#v\n", name, pid, response)
	return &response, nil
}
