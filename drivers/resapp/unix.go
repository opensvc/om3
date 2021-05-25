// +build !windows

package resapp

import (
	"bufio"
	"fmt"
	"io"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/utilexec"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// T is the driver structure for app unix & linux.
type T struct {
	BaseT
	Path         path.T         `json:"path"`
	Nodes        []string       `json:"nodes"`
	ScriptPath   string         `json:"script"`
	StartCmd     []string       `json:"start"`
	StopCmd      []string       `json:"stop"`
	CheckCmd     []string       `json:"check"`
	InfoCmd      []string       `json:"info"`
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
	cmd := t.GetCmd(t.StopCmd, "stop")
	if cmd == nil {
		return nil
	}
	t.Log().Info().Msgf("running %s", cmd.String())
	return t.RunOutErr(cmd)
}

// Status evaluates and display the Resource status and logs
func (t *T) Status() status.T {
	cmd := t.GetCmd(t.CheckCmd, "status")
	t.Log().Debug().Msgf("Status() running %s", cmd.String())
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

func (t T) logInfo(r io.Reader, done chan bool) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		t.Log().Info().Msgf("| %v", s.Text())
	}
	done <- true
}

func (t T) logWarn(r io.Reader, done chan bool) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		t.Log().Error().Msgf("| %v", s.Text())
	}
	done <- true
}

func (t T) RunOutErr(cmd *exec.Cmd) (err error) {
	var stdout, stderr io.ReadCloser
	closer := func(c io.Closer) {
		_ = c.Close()
	}
	if t.Cwd != "" {
		t.Log().Debug().Msgf("run command from %v", t.Cwd)
		cmd.Dir = t.Cwd
	}
	if err := utilexec.SetCredential(cmd, t.User, t.Group); err != nil {
		t.Log().Error().Err(err).Msgf("unable to set credential from user '%v', group '%v'", t.User, t.Group)
		return err
	}

	if stdout, err = cmd.StdoutPipe(); err != nil {
		return err
	}
	defer closer(stdout)
	if stderr, err = cmd.StderrPipe(); err != nil {
		return err
	}
	defer closer(stderr)
	infoChan := make(chan bool)
	errChan := make(chan bool)
	go t.logInfo(stdout, infoChan)
	go t.logWarn(stderr, errChan)

	if err = cmd.Start(); err != nil {
		return err
	}
	// wait for log watchers done
	<-infoChan
	<-errChan

	if err = cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func (t T) GetCmd(command []string, action string) (cmd *exec.Cmd) {
	if len(command) == 1 {
		scriptCommand := command[0]
		if scriptCommandBool, err := converters.ToBool(scriptCommand); err == nil {
			switch scriptCommandBool {
			case true:
				commandStrings := []string{t.getScript()}
				commandStrings = append(commandStrings, action)
				cmd = Command(commandStrings)
			case false:
				return
			}
		} else {
			cmd = Command(command)
		}
	} else {
		cmd = Command(command)
	}
	if cmd == nil {
		return
	}
	if len(t.Env) > 0 {
		cmd.Env = append([]string{}, t.Env...)
	}
	return
}

// getScript return script kw value
// when script is a basename:
//   <pathetc>/namespaces/<namespace>/<kind>/<svcname>.d/<script> (when namespace is not root)
//   <pathetc>/<svcname>.d/<script> (when namespace is root)
//
func (t T) getScript() string {
	s := t.ScriptPath
	if s[0] == os.PathSeparator {
		return s
	}
	var p string
	if t.Path.Namespace != "root" {
		p = fmt.Sprintf("%s/namespaces/%s/%s/%s.d/%s", config.Node.Paths.Etc, t.Path.Namespace, t.Path.Kind, t.Path.Name, s)
	} else {
		p = fmt.Sprintf("%s/%s.d/%s", config.Node.Paths.Etc, t.Path.Name, s)
	}
	return filepath.FromSlash(p)
}
