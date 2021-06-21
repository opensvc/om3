// +build !windows

package resapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/funcopt"
)

// T is the driver structure for app unix & linux.
type T struct {
	BaseT
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

func (t T) SortKey() string {
	if len(t.StartCmd) > 1 && isSequenceNumber(t.StartCmd) {
		return t.StartCmd + " " + t.RID()
	} else {
		return t.RID() + " " + t.RID()
	}
}

func (t T) Abort() bool {
	return false
}

// Stop the Resource
func (t T) Stop(ctx context.Context) (err error) {
	t.Log().Debug().Msg("Stop()")
	var opts []funcopt.O
	if opts, err = t.GetFuncOpts(t.StopCmd, "stop"); err != nil {
		return err
	}
	if len(opts) == 0 {
		return nil
	}

	opts = append(opts,
		command.WithLogger(t.Log()),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.WarnLevel),
		command.WithTimeout(t.GetTimeout("stop")),
	)
	cmd := command.New(opts...)

	appStatus := t.Status()
	if appStatus == status.Down {
		t.Log().Info().Msg("already down")
		return nil
	}

	t.Log().Info().Msgf("running %s", cmd.String())
	return cmd.Run()
}

// Status evaluates and display the Resource status and logs
func (t *T) Status() status.T {
	t.Log().Debug().Msg("status()")
	var opts []funcopt.O
	var err error
	if opts, err = t.GetFuncOpts(t.CheckCmd, "check"); err != nil {
		t.Log().Error().Err(err).Msg("GetFuncOpts")
		if t.StatusLogKw {
			t.StatusLog().Error("prepareXcmd %v", err.Error())
		}
		return status.Undef
	}
	if len(opts) == 0 {
		return status.NotApplicable
	}

	opts = append(opts,
		command.WithLogger(t.Log()),
		command.WithStdoutLogLevel(zerolog.Disabled),
		command.WithStderrLogLevel(zerolog.Disabled),
		command.WithTimeout(t.GetTimeout("check")),
	)
	if t.StatusLogKw {
		opts = append(opts, command.WithOnStdoutLine(func(s string) { t.StatusLog().Info(s) }))
		opts = append(opts, command.WithOnStderrLine(func(s string) { t.StatusLog().Warn(s) }))
	}
	cmd := command.New(opts...)

	t.Log().Debug().Msgf("Status() running %s", cmd.String())
	if err = cmd.Start(); err != nil {
		return status.Undef
	}
	if err = cmd.Wait(); err != nil {
		t.Log().Debug().Msg("status is down")
		return status.Down
	}
	t.Log().Debug().Msgf("status is up")
	return status.Up
}

func (t T) Provision(ctx context.Context) error {
	return nil
}

func (t T) Unprovision(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

// GetFuncOpts returns
func (t T) GetFuncOpts(s string, action string) ([]funcopt.O, error) {
	var err error
	if len(s) == 0 {
		t.Log().Debug().Msgf("nothing to do for action '%v'", action)
		return nil, nil
	}
	var baseCommand string
	if baseCommand, err = t.getCmdStringFromBoolRule(s, action); err != nil {
		return nil, err
	}
	if len(baseCommand) == 0 {
		t.Log().Debug().Msgf("no basecommand for action '%v'", action)
		return nil, nil
	}
	limitCommands := command.ShLimitCommands(t.toLimits())
	if len(limitCommands) > 0 {
		baseCommand = limitCommands + " && " + baseCommand
	}
	var cmdArgs []string
	if cmdArgs, err = command.CmdArgsFromString(baseCommand); err != nil {
		t.Log().Error().Err(err).Msgf("unable to CmdArgsFromString for action '%v'", action)
		return nil, err
	}
	var env []string
	env, err = t.getEnv()
	if err != nil {
		t.Log().Error().Err(err).Msgf("unable to get environment for action '%v'", action)
		return nil, err
	}
	options := []funcopt.O{
		command.WithName(cmdArgs[0]),
		command.WithArgs(cmdArgs[1:]),
		command.WithUser(t.User),
		command.WithGroup(t.Group),
		command.WithCWD(t.Cwd),
		command.WithEnv(env),
	}
	return options, nil
}

// getCmdStringFromBoolRule get command string for 'action' using bool rule on 's'
// if 's' is a
//   true like => getScript() + " " + action
//   false like => ""
//   other => original value
func (t T) getCmdStringFromBoolRule(s string, action string) (string, error) {
	if scriptCommandBool, ok := boolRule(s); ok {
		switch scriptCommandBool {
		case true:
			scriptValue := t.getScript()
			if scriptValue == "" {
				t.Log().Warn().Msgf("action '%v' as true value but 'script' keyword is empty", action)
				return "", fmt.Errorf("unable to get script value")
			}
			return scriptValue + " " + action, nil
		case false:
			return "", nil
		}
	}
	return s, nil
}

// getScript return script kw value
// when script is a basename:
//   <pathetc>/namespaces/<namespace>/<kind>/<svcname>.d/<script> (when namespace is not root)
//   <pathetc>/<svcname>.d/<script> (when namespace is root)
//
func (t T) getScript() string {
	s := t.ScriptPath
	if len(s) == 0 {
		return ""
	}
	if s[0] == os.PathSeparator {
		return s
	}
	var p string
	if t.Path.Namespace != "root" {
		p = fmt.Sprintf("%s/namespaces/%s/%s/%s.d/%s", rawconfig.Node.Paths.Etc, t.Path.Namespace, t.Path.Kind, t.Path.Name, s)
	} else {
		p = fmt.Sprintf("%s/%s.d/%s", rawconfig.Node.Paths.Etc, t.Path.Name, s)
	}
	return filepath.FromSlash(p)
}

// boolRule return bool, ok
// detect if s is a bool like, or sequence number
func boolRule(s string) (bool, bool) {
	if v, err := converters.Bool.Convert(s); err == nil {
		return v.(bool), true
	}
	if isSequenceNumber(s) {
		return true, true
	}
	return false, false
}

func isSequenceNumber(s string) bool {
	if len(s) < 2 {
		return false
	}
	if _, err := strconv.ParseInt(s, 10, 16); err == nil {
		return true
	}
	return false
}

func (t T) GetTimeout(action string) time.Duration {
	var timeout *time.Duration
	switch action {
	case "start":
		timeout = t.StartTimeout
	case "stop":
		timeout = t.StopTimeout
	case "check":
		timeout = t.CheckTimeout
	case "info":
		timeout = t.InfoTimeout
	}
	if timeout == nil {
		timeout = t.Timeout
	}
	if timeout == nil {
		return 0
	}
	return *timeout
}
