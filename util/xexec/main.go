package xexec

import (
	"bufio"
	"context"
	"github.com/anmitsu/go-shlex"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// T struct hold attributes that needs to be applied on exec.Cmd by Update func
type T struct {
	Cwd        string
	CmdArgs    []string
	CmdEnv     []string
	Credential *syscall.Credential
}

// Update func set attributes on existing exec.Cmd 'cmd' from T struct settings
func (t T) Update(cmd *exec.Cmd) error {
	if cmd == nil {
		return errors.New("Can't Update nil cmd")
	}
	if t.Cwd != "" {
		cmd.Dir = t.Cwd
	}
	if t.Credential != nil {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Credential = t.Credential
	}
	if len(t.CmdEnv) > 0 {
		cmd.Env = append(cmd.Env, t.CmdEnv...)
	}
	return nil
}

// CommandFromString wrapper to exec.Command from a string command 's'
// When string command 's' contains multiple commands,
//   exec.Command("/bin/sh", "-c", s)
// else
//   exec.Command from shlex.Split(s)
func CommandFromString(s string) (*exec.Cmd, error) {
	args, err := commandArgsFromString(s)
	if err != nil {
		return nil, err
	}
	return exec.Command(args[0], args[1:]...), nil
}

type (
	texter interface {
		Text() string
	}
	bytter interface {
		Bytes() []byte
	}
	Bytetexter interface {
		texter
		bytter
	}
	doOuter interface {
		DoOut(Bytetexter, int)
	}
	doErrer interface {
		DoErr(Bytetexter, int)
	}
)

type Cmd struct {
	*exec.Cmd
	Log *zerolog.Logger

	watch     interface{}
	done      chan bool
	goroutine []func()
	ctx       context.Context
	duration  time.Duration
	cancel    func()
	pid       int
}

// NewCmd Create a helper to call Start() and Wait()
// with logs.
func NewCmd(log *zerolog.Logger, cmd *exec.Cmd, watch interface{}) *Cmd {
	return &Cmd{
		Log:   log,
		Cmd:   cmd,
		watch: watch,
	}
}

// SetDuration on a Cmd
func (c *Cmd) SetDuration(d time.Duration) {
	c.duration = d
}

// Start is like exec.Cmd Start()
//
// When 'watch' is a doOuter, stdout entries are scaned and
// sent to watch.(DoOouter).DoOut
//
// When 'watch' is a doErrer, stderr entries are scaned and
// sent to watch.(doErrer).Doerr
//
func (c *Cmd) Start() (err error) {
	cmd := c.Cmd
	log := c.Log
	watch := c.watch
	closer := func(c io.Closer) {
		_ = c.Close()
	}
	if w, ok := watch.(doOuter); ok {
		var r io.ReadCloser
		if r, err = cmd.StdoutPipe(); err != nil {
			return err
		}
		c.goroutine = append(c.goroutine, func() {
			defer closer(r)
			s := bufio.NewScanner(r)
			for s.Scan() {
				w.DoOut(s, cmd.Process.Pid)
			}
			c.done <- true
		})
	}
	if w, ok := watch.(doErrer); ok {
		var r io.ReadCloser
		if r, err = cmd.StderrPipe(); err != nil {
			return err
		}
		c.goroutine = append(c.goroutine, func() {
			defer closer(r)
			s := bufio.NewScanner(r)
			for s.Scan() {
				w.DoErr(s, cmd.Process.Pid)
			}
			c.done <- true
		})
	}
	if c.duration > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), c.duration)
		c.ctx = ctx
		c.cancel = cancel
		c.Log.Info().Msgf("run with duration=%v", ctx)
		deadLineDone := false
		c.goroutine = append(c.goroutine, func() {
			select {
			case <-ctx.Done():
				if deadLineDone {
					return
				}
				deadLineDone = true
				err := ctx.Err()
				if err == context.DeadlineExceeded {

					if w, ok := watch.(doErrer); ok {
						w.DoErr(msg{"DeadlineExceeded"}, cmd.Process.Pid)
					}
					if cmd.Process != nil {
						c.Log.Debug().Err(err).Str("cmd", cmd.String()).Int("pid", cmd.Process.Pid).Msg("DeadlineExceeded")
						err := cmd.Process.Kill()
						if err != nil {
							c.Log.Debug().Msg("kill proc failed")
						}
					} else {
						c.Log.Debug().Err(err).Msgf("DeadlineExceeded, but cmd.Process is nil")
					}
				}
			}
			c.done <- true
		})
	}
	log.Debug().Str("cmd", cmd.String()).Msg("cmd.Start()")
	if err = cmd.Start(); err != nil {
		log.Debug().Err(err).Msgf("cmd.Start() %v,", cmd)
		return err
	}

	if len(c.goroutine) > 0 {
		c.done = make(chan bool, len(c.goroutine))
		for _, f := range c.goroutine {
			go f()
		}
	}
	return nil
}

func (c *Cmd) Wait() error {
	waitCount := len(c.goroutine)
	if c.cancel != nil {
		waitCount = waitCount - 1
		defer c.cancel()
	}
	// wait for of goroutines
	for i := 0; i < waitCount; i++ {
		<-c.done
	}
	msg := "cmd.Wait()"
	cmd := c.Cmd
	if err := cmd.Wait(); err != nil {
		cmd.ProcessState.ExitCode()
		c.Log.Debug().Err(err).Str("cmd", cmd.String()).Int("exitCode", cmd.ProcessState.ExitCode()).Msg(msg)
		return err
	}
	c.Log.Debug().Str("cmd", cmd.String()).Int("exitCode", cmd.ProcessState.ExitCode()).Msg(msg)
	return nil
}

type msg struct {
	text string
}

func (m msg) Text() string {
	return m.text
}

func (m msg) Bytes() []byte {
	return []byte(m.text)
}

func CommandArgsFromString(s string) ([]string, error) {
	return commandArgsFromString(s)
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
