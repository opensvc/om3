//go:build !windows

package resapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/statusbus"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/pg"
	"github.com/opensvc/om3/util/ulimit"
)

type (
	// T is the driver structure for app unix & linux.
	T struct {
		BaseT
		Path         naming.Path    `json:"path"`
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
		PG           pg.Config      `json:"pg"`
		Limit        ulimit.Config  `json:"limit"`
	}

	statuser interface {
		Status(context.Context) status.T
	}
)

var (
	baseExitToStatusMap = map[int]status.T{
		0: status.Up,
		1: status.Down,
	}
	stringToStatus = map[string]status.T{
		"up":    status.Up,
		"down":  status.Down,
		"warn":  status.Warn,
		"n/a":   status.NotApplicable,
		"undef": status.Undef,
	}
)

func (t T) SortKey() string {
	if len(t.StartCmd) > 1 && isSequenceNumber(t.StartCmd) {
		return t.StartCmd + " " + t.RID()
	} else {
		return t.RID() + " " + t.RID()
	}
}

// CommonStop stops the Resource
func (t *T) CommonStop(ctx context.Context, r statuser) (err error) {
	var opts []funcopt.O
	if opts, err = t.GetFuncOpts(t.StopCmd, "stop"); err != nil {
		t.Errorf("prepare 'stop' command: %s", err)
		if t.StatusLogKw {
			t.StatusLog().Error("prepare cmd %s", err)
		}
		return err
	}
	if len(opts) == 0 {
		return nil
	}

	opts = append(opts,
		command.WithLogger(t.Log()),
		command.WithLogPrefix(t.Msgf("")+": "),
		command.WithErrorExitCodeLogLevel(zerolog.WarnLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.WarnLevel),
		command.WithTimeout(t.GetTimeout("stop")),
	)
	cmd := command.New(opts...)

	appStatus := r.Status(ctx)
	if appStatus == status.Down {
		t.Infof("already down")
		return nil
	}

	t.Infof("run: %s", cmd)
	return cmd.Run()
}

func (t *T) isInstanceSufficientlyStarted(ctx context.Context) bool {
	sb := statusbus.FromContext(ctx)
	o := t.GetObjectDriver()
	l := o.ResourcesByDrivergroups([]driver.Group{
		driver.GroupIP,
		driver.GroupFS,
		driver.GroupShare,
		driver.GroupDisk,
		driver.GroupContainer,
	})
	for _, r := range l {
		switch r.ID().DriverGroup() {
		case driver.GroupIP:
		case driver.GroupFS:
		case driver.GroupShare:
		case driver.GroupDisk:
			switch r.Manifest().DriverID.Name {
			case "drbd":
				continue
			case "scsireserv":
				continue
			}
		case driver.GroupContainer:
		default:
			continue
		}
		st := sb.Get(r.RID())
		switch st {
		case status.Up:
		case status.NotApplicable:
		default:
			// required resource is not up
			t.StatusLog().Info("not evaluated (%s is %s)", r.RID(), st)
			return false
		}
	}
	return true
}

// CommonStatus evaluates and display the Resource status and logs
func (t *T) CommonStatus(ctx context.Context) status.T {
	var opts []funcopt.O
	var err error
	if opts, err = t.GetFuncOpts(t.CheckCmd, "check"); err != nil {
		t.Errorf("prepare 'status' command: %s", err)
		if t.StatusLogKw {
			t.StatusLog().Error("prepare cmd %s", err)
		}
		return status.Undef
	}
	if len(opts) == 0 {
		return status.NotApplicable
	}
	if !t.isInstanceSufficientlyStarted(ctx) {
		return status.NotApplicable
	}

	opts = append(opts,
		command.WithLogger(t.Log()),
		command.WithLogPrefix(t.Msgf("")+": "),
		command.WithStdoutLogLevel(zerolog.Disabled),
		command.WithStderrLogLevel(zerolog.Disabled),
		command.WithTimeout(t.GetTimeout("check")),
		command.WithIgnoredExitCodes(),
	)
	if t.StatusLogKw {
		opts = append(opts, command.WithOnStdoutLine(func(s string) { t.StatusLog().Info(s) }))
		opts = append(opts, command.WithOnStderrLine(func(s string) { t.StatusLog().Warn(s) }))
	}
	cmd := command.New(opts...)

	t.Debugf("status running command: %s", cmd.String())
	if err = cmd.Start(); err != nil {
		return status.Undef
	}
	if err = cmd.Wait(); err != nil {
		t.Debugf("status is down")
		return status.Down
	}
	resultStatus, err := t.ExitCodeToStatus(cmd.ExitCode())
	if err != nil {
		t.StatusLog().Warn("%s", err)
	}
	t.Debugf("status result: %v", resultStatus)
	return resultStatus
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

func (t T) BaseCmdArgs(s string, action string) ([]string, error) {
	var err error
	var baseCommand string
	if baseCommand, err = t.getCmdStringFromBoolRule(s, action); err != nil {
		return nil, err
	}
	if len(baseCommand) == 0 {
		t.Debugf("no base command for action '%v'", action)
		return nil, nil
	}
	return command.CmdArgsFromString(baseCommand)
}

// CmdArgs returns the command argv of an action
func (t T) CmdArgs(s string, action string) ([]string, error) {
	if len(s) == 0 {
		t.Debugf("nothing to do for action '%v'", action)
		return nil, nil
	}
	baseCommandSlice, err := t.BaseCmdArgs(s, action)
	if err != nil {
		return nil, err
	}
	wrapArgs := t.toCaps().Argv()
	prog := ""
	if prog, err = os.Executable(); err != nil {
		return nil, fmt.Errorf("lookup prog: %w", err)
	}
	if len(wrapArgs) > 0 {
		wrap := append([]string{prog, "exec"}, wrapArgs...)
		wrap = append(wrap, "--")
		return append(wrap, baseCommandSlice...), nil
	}
	return baseCommandSlice, nil
}

// GetFuncOpts returns a list of functional options to use with command.New()
func (t T) GetFuncOpts(s string, action string) ([]funcopt.O, error) {
	cmdArgs, err := t.CmdArgs(s, action)
	if err != nil || cmdArgs == nil {
		return nil, err
	}
	env, err := t.getEnv()
	if err != nil {
		return nil, err
	}
	if len(cmdArgs) == 0 {
		return nil, fmt.Errorf("no command for action %s", action)
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

func (t T) Info(ctx context.Context) (resource.InfoKeys, error) {
	durationToString := func(duration *time.Duration) string {
		if duration == nil {
			return ""
		}
		return duration.String()
	}
	result := append(
		resource.InfoKeys{},
		resource.InfoKey{"script", t.ScriptPath},
		resource.InfoKey{"start", t.StartCmd},
		resource.InfoKey{"stop", t.StopCmd},
		resource.InfoKey{"check", t.CheckCmd},
		resource.InfoKey{"info", t.InfoCmd},
		resource.InfoKey{"timeout", durationToString(t.Timeout)},
		resource.InfoKey{"start_timeout", durationToString(t.StartTimeout)},
		resource.InfoKey{"stop_timeout", durationToString(t.StopTimeout)},
		resource.InfoKey{"check_timeout", durationToString(t.CheckTimeout)},
		resource.InfoKey{"info_timeout", durationToString(t.InfoTimeout)},
	)
	var opts []funcopt.O
	var err error
	if opts, err = t.GetFuncOpts(t.InfoCmd, "info"); err != nil {
		t.Errorf("prepare 'info' command: %s", err)
		if t.StatusLogKw {
			t.StatusLog().Error("prepare cmd %s", err)
		}
		return nil, err
	}
	if len(opts) == 0 {
		return result, nil
	}

	opts = append(opts,
		command.WithLogger(t.Log()),
		command.WithLogPrefix(t.Msgf("")+": "),
		command.WithTimeout(t.GetTimeout("info")),
		command.WithBufferedStdout(),
	)
	cmd := command.New(opts...)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	lines := strings.Split(string(cmd.Stdout()), "\n")
	for _, line := range lines {
		lineSplit := strings.Split(line, ":")
		if len(lineSplit) != 2 {
			continue
		}
		key := strings.Trim(lineSplit[0], "\n ")
		value := strings.Trim(lineSplit[1], "\n ")
		result = append(result, resource.InfoKey{key, value})
	}
	return result, nil
}

// getCmdStringFromBoolRule get command string for 'action' using bool rule on 's'
// if 's' is a
//
//	true like => getScript() + " " + action
//	false like => ""
//	other => original value
func (t T) getCmdStringFromBoolRule(s string, action string) (string, error) {
	if scriptCommandBool, ok := boolRule(s); ok {
		switch scriptCommandBool {
		case true:
			scriptValue := t.getScript()
			if scriptValue == "" {
				t.Warnf("action '%v' as true value but 'script' keyword is empty", action)
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
//
//	<pathetc>/namespaces/<namespace>/<kind>/<svcname>.d/<script> (when namespace is not root)
//	<pathetc>/<svcname>.d/<script> (when namespace is root)
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
		p = fmt.Sprintf("%s/namespaces/%s/%s/%s.d/%s", rawconfig.Paths.Etc, t.Path.Namespace, t.Path.Kind, t.Path.Name, s)
	} else {
		p = fmt.Sprintf("%s/%s.d/%s", rawconfig.Paths.Etc, t.Path.Name, s)
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

func (t T) ExitCodeToStatus(exitCode int) (status.T, error) {
	convertMap, err := t.exitCodeToStatusMap()
	if s, ok := convertMap[exitCode]; ok {
		return s, err
	}
	return status.Warn, err
}

// exitCodeToStatusMap return exitCodeToStatus map
//
// invalid entry rules are dropped
func (t T) exitCodeToStatusMap() (map[int]status.T, error) {
	if len(t.RetCodes) == 0 {
		return baseExitToStatusMap, nil
	}
	dropMessages := make([]string, 0)
	m := make(map[int]status.T)
	for _, rule := range strings.Fields(t.RetCodes) {
		dropMessage := fmt.Sprintf("retcodes invalid rule '%v'", rule)
		ruleSplit := strings.Split(rule, ":")
		if len(ruleSplit) != 2 {
			dropMessages = append(dropMessages, dropMessage)
			continue
		}
		code, err := strconv.Atoi(ruleSplit[0])
		if err != nil {
			dropMessages = append(dropMessages, dropMessage)
			continue
		}
		statusValue, ok := stringToStatus[ruleSplit[1]]
		if !ok {
			dropMessages = append(dropMessages, dropMessage)
			continue
		}
		m[code] = statusValue
	}
	var err error
	if len(dropMessages) > 0 {
		err = fmt.Errorf("%s", strings.Join(dropMessages, "\n"))
	}
	if len(m) == 0 {
		return baseExitToStatusMap, err
	}
	return m, err
}
