package resappunix

import (
	"bufio"
	"io"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/app/resappbase"
	"os/exec"
	"time"
)

// T is the driver structure for app unix & linux.
type T struct {
	resappbase.T
	Path         path.T         `json:"path"`
	Nodes        []string       `json:"nodes"`
	ScriptPath   string         `json:"script"`
	StartCmd     string         `json:"start"`
	StopCmd      string         `json:"stop"`
	CheckCmd     string         `json:"check"`
	InfoCmd      string         `json:"info"`
	StatusLogKw  bool           `json:"status_log"`
	CheckTimeout *time.Duration `json:"check_timeout"`
	InfoTimeout  *time.Duration `json:"info_timeout"`
	Cwd          string         `json:"cwd"`
	User         string         `json:"user"`
	Group        string         `json:"group"`
	LimitAs      *int64         `json:"limit_as"`
	LimitCpu     *time.Duration `json:"limit_cpu"`
	LimitCore    *int64         `json:"limit_core"`
	LimitData    *int64         `json:"limit_data"`
	LimitFSize   *int64         `json:"limit_fsize"`
	LimitMemLock *int64         `json:"limit_memlock"`
	LimitNoFile  *int64         `json:"limit_nofile"`
	LimitNProc   *int64         `json:"limit_nproc"`
	LimitRss     *int64         `json:"limit_rss"`
	LimitStack   *int64         `json:"limit_stack"`
	LimitVMem    *int64         `json:"limit_vmem"`
}

func (t T) Abort() bool {
	return false
}

// Stop the Resource
func (t T) Stop() error {
	t.Log().Debug().Msg("Stop()")
	appStatus := t.Status()
	if appStatus == status.Down {
		t.Log().Info().Msg("already down")
		return nil
	}
	t.Log().Info().Msgf("running %s", t.StopCmd)
	cmd := t.GetCmd(t.StopCmd)
	return t.RunOutErr(cmd)
}

// Status evaluates and display the Resource status and logs
func (t *T) Status() status.T {
	t.Log().Debug().Msgf("Status() running %s", t.CheckCmd)
	cmd := t.GetCmd(t.CheckCmd)
	err := cmd.Run()
	if err != nil {
		t.Log().Debug().Msg("status is down")
		return status.Down
	}
	t.Log().Debug().Msgf("status is up")
	return status.Up
}

func (t T) Provision() error {
	return nil
}

func (t T) Unprovision() error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t T) logInfo(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		t.Log().Info().Msgf("| %v", s.Text())
	}
}

func (t T) logWarn(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		t.Log().Error().Msgf("| %v", s.Text())
	}
}

func (t T) RunOutErr(cmd *exec.Cmd) (err error) {
	var stdout, stderr io.ReadCloser
	closer := func(c io.Closer) {
		_ = c.Close()
	}
	if stdout, err = cmd.StdoutPipe(); err != nil {
		return err
	}
	defer closer(stdout)
	if stderr, err = cmd.StderrPipe(); err != nil {
		return err
	}
	defer closer(stderr)

	if err = cmd.Start(); err != nil {
		return err
	}
	go t.logInfo(stdout)
	go t.logWarn(stderr)

	if err = cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func (t T) GetCmd(command string) *exec.Cmd {
	cmd := resappbase.Command(command)
	if len(t.Env) > 0 {
		cmd.Env = append([]string{}, t.Env...)
	}
	return cmd
}
