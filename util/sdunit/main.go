// Package sdunit runs a command in a transient systemd service unit, whose
// properties sandbox it, and ends the processes of the unit without queuing
// a stop job.
//
// A stop job is never queued, as systemctl stop would: at a reboot, the stop
// of the units is one transaction, where the stop job of a unit ordered
// before the one of the om daemon waits for the daemon to stop, while the
// daemon waits for the command stopping the unit. The processes of a unit
// are signaled instead, which is no job, and the unit becomes inactive once
// they are gone.
package sdunit

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type (
	// Property is a unit property, as systemd-run -p takes it.
	Property struct {
		Key   string
		Value string
	}

	// Run is a command to run in a transient service unit.
	Run struct {
		// Unit is the name of the service unit.
		Unit string

		// Slice is the slice unit the service is placed in, empty for
		// the default one.
		Slice string

		// Description is the description of the unit.
		Description string

		// Type is the type of the service: notify, forking or exec.
		Type string

		// Wait makes systemd-run wait for the command to end, its
		// standard input and outputs connected to the ones of
		// systemd-run, and exit with its exit code.
		Wait bool

		// Properties are the properties of the configuration, the
		// sandbox of the command.
		Properties []Property

		// OwnProperties are the properties om sets.
		OwnProperties []Property

		// Argv is the command.
		Argv []string
	}

	// State is the state of a unit, as systemctl show reports it.
	State struct {
		ActiveState  string
		Result       string
		MainPID      int
		ControlGroup string
		InvocationID string
	}
)

// ServerUnit is the unit of the om daemon, the units of the commands are
// ordered before: their stop jobs, at a reboot, come after the one of the
// daemon, which stops the commands first, in the order of the resources.
const ServerUnit = "opensvc-server.service"

var (
	// propertyKeyRE matches the start of a property, its key and "=".
	propertyKeyRE = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*=`)

	// refusedKeys are the properties om sets itself, or which would make
	// the unit escape what om controls of it.
	refusedKeys = map[string]string{
		"Type":                  "om sets the type of the service",
		"User":                  "set the user keyword: om exec changes the identity",
		"Group":                 "set the group keyword: om exec changes the identity",
		"DynamicUser":           "set the user keyword: om exec changes the identity",
		"UMask":                 "set the umask keyword",
		"WorkingDirectory":      "set the cwd keyword",
		"RootDirectory":         "the command runs om exec, which the node holds",
		"RootImage":             "the command runs om exec, which the node holds",
		"Slice":                 "om places the unit in the process group of the resource",
		"Environment":           "set the environment keyword",
		"EnvironmentFile":       "set the environment keyword",
		"PassEnvironment":       "set the environment keyword",
		"UnsetEnvironment":      "set the environment keyword",
		"KillMode":              "om ends the processes of the unit",
		"RemainAfterExit":       "om sets how the unit ends",
		"PIDFile":               "om sets how the unit tracks its processes",
		"GuessMainPID":          "om sets how the unit tracks its processes",
		"NotifyAccess":          "om sets how the unit tells it started",
		"NoNewPrivileges":       "set the no_new_privileges keyword: om exec applies it after the user and the limits",
		"CapabilityBoundingSet": "set the capability_bounding_set keyword: om exec needs its capabilities to set the user and the limits",
		"AmbientCapabilities":   "om exec clears the inheritable capabilities, so the ambient ones are lost",
		"DefaultDependencies":   "om orders the unit before the om daemon",
		"After":                 "om orders the unit before the om daemon",
		"Before":                "om orders the unit before the om daemon",
		"Requires":              "a dependency would start or stop the unit with another",
		"Requisite":             "a dependency would start or stop the unit with another",
		"Wants":                 "a dependency would start or stop the unit with another",
		"BindsTo":               "a dependency would start or stop the unit with another",
		"PartOf":                "a dependency would start or stop the unit with another",
		"Upholds":               "a dependency would start or stop the unit with another",
		"Conflicts":             "a dependency would start or stop the unit with another",
		"OnFailure":             "a dependency would start or stop the unit with another",
		"OnSuccess":             "a dependency would start or stop the unit with another",
		"PropagatesStopTo":      "a dependency would start or stop the unit with another",
		"StopPropagatedFrom":    "a dependency would start or stop the unit with another",
	}

	// refusedPrefixes are the families of properties om sets itself: the
	// commands, the restarts, the timeouts and the limits.
	refusedPrefixes = map[string]string{
		"Exec":          "the command is the one of the resource",
		"Restart":       "the daemon restarts the resources",
		"Timeout":       "set the timeout keywords of the resource",
		"Limit":         "set the limit_* keywords: om exec applies them",
		"StandardInput": "om connects the standard input of the command",
	}
)

func (t Property) String() string {
	return t.Key + "=" + t.Value
}

// Parse returns the properties of s, as the systemd_properties keyword sets
// them: words separated by blanks, quoted as in a shell, a word starting
// with a property name and = starting a property, the others added to the
// value of the property before them, separated by a space. So
//
//	ProtectSystem=strict ReadWritePaths=/srv/data /var/log/app PrivateTmp=yes
//
// is three properties, the second of two paths.
func Parse(s string) ([]Property, error) {
	words, err := splitWords(s)
	if err != nil {
		return nil, err
	}
	var l []Property
	for _, word := range words {
		if propertyKeyRE.MatchString(word) {
			key, value, _ := strings.Cut(word, "=")
			l = append(l, Property{Key: key, Value: value})
			continue
		}
		if len(l) == 0 {
			return nil, fmt.Errorf("%q is no property: a property is written <Name>=<value>, as ProtectSystem=strict", word)
		}
		p := &l[len(l)-1]
		if p.Value == "" {
			p.Value = word
		} else {
			p.Value += " " + word
		}
	}
	return l, nil
}

// Validate refuses s when it is not a list of properties, or names one om
// sets itself.
func Validate(s string) error {
	l, err := Parse(s)
	if err != nil {
		return err
	}
	var errs []error
	for _, p := range l {
		if reason, ok := Refused(p.Key); ok {
			errs = append(errs, fmt.Errorf("%s is refused: %s", p.Key, reason))
		}
	}
	return errors.Join(errs...)
}

// Refused returns why the property key is refused, if it is.
func Refused(key string) (string, bool) {
	if reason, ok := refusedKeys[key]; ok {
		return reason, true
	}
	for prefix, reason := range refusedPrefixes {
		if strings.HasPrefix(key, prefix) {
			return reason, true
		}
	}
	return "", false
}

// splitWords splits s in words separated by blanks, a quoted part, single or
// double, keeping its blanks, a backslash escaping the next character out of
// single quotes.
func splitWords(s string) ([]string, error) {
	var (
		words   []string
		word    strings.Builder
		inWord  bool
		quote   rune
		escaped bool
	)
	for _, r := range s {
		switch {
		case escaped:
			word.WriteRune(r)
			escaped = false
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\\':
			escaped = true
			inWord = true
		case quote == '"':
			if r == '"' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inWord = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote", quote)
	}
	if escaped {
		return nil, fmt.Errorf("trailing backslash")
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}

// envNameRE matches a valid name of an environment variable.
var envNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// EnvFile renders env, a list of NAME=VALUE, as an environment file systemd
// reads, each value double quoted with its \, ", $ and ` escaped, so any
// value comes back as it is, its newlines included. A variable whose name
// systemd refuses is an error.
func EnvFile(env []string) ([]byte, error) {
	var b bytes.Buffer
	for _, s := range env {
		name, value, ok := strings.Cut(s, "=")
		if !ok {
			continue
		}
		if !envNameRE.MatchString(name) {
			return nil, fmt.Errorf("environment variable %q: invalid name", name)
		}
		b.WriteString(name)
		b.WriteString(`="`)
		for _, r := range value {
			switch r {
			case '\\', '"', '$', '`':
				b.WriteRune('\\')
			}
			b.WriteRune(r)
		}
		b.WriteString("\"\n")
	}
	return b.Bytes(), nil
}

// EscapeArg returns s as a word of the command of a unit, which systemd
// expands the variables of: a $ is doubled to stay one.
func EscapeArg(s string) string {
	return strings.ReplaceAll(s, "$", "$$")
}

// Args returns the systemd-run command running t.
func (t Run) Args() []string {
	args := []string{"systemd-run", "--unit=" + t.Unit, "--quiet"}
	if t.Slice != "" {
		args = append(args, "--slice="+t.Slice)
	}
	if t.Description != "" {
		args = append(args, "--description="+t.Description)
	}
	if t.Wait {
		args = append(args, "--wait", "--pipe")
	}
	if t.Type != "" {
		args = append(args, "--property=Type="+t.Type)
	}
	for _, p := range t.OwnProperties {
		args = append(args, "--property="+p.String())
	}
	for _, p := range t.Properties {
		args = append(args, "--property="+p.String())
	}
	args = append(args, "--")
	for _, s := range t.Argv {
		args = append(args, EscapeArg(s))
	}
	return args
}

// IsActive says the unit runs processes, or is about to.
func (t State) IsActive() bool {
	switch t.ActiveState {
	case "active", "activating", "deactivating", "reloading", "refreshing":
		return true
	}
	return false
}

// systemctl runs systemctl with args, and returns its output.
var systemctl = func(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return b, fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return b, nil
}

// GetState returns the state of the unit, inactive for a unit not loaded.
func GetState(ctx context.Context, unit string) (State, error) {
	var st State
	b, err := systemctl(ctx, "show", unit, "--property=ActiveState,Result,MainPID,ControlGroup,InvocationID")
	if err != nil {
		return st, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		switch key {
		case "ActiveState":
			st.ActiveState = value
		case "Result":
			st.Result = value
		case "MainPID":
			st.MainPID, _ = strconv.Atoi(value)
		case "ControlGroup":
			st.ControlGroup = value
		case "InvocationID":
			st.InvocationID = value
		}
	}
	return st, scanner.Err()
}

// cgroupRoots are where the cgroup of a unit is, by the hierarchy systemd
// tracks its processes in: the unified one, the one of a hybrid node, and
// the named systemd one of a node of the first version.
var cgroupRoots = []string{"/sys/fs/cgroup", "/sys/fs/cgroup/unified", "/sys/fs/cgroup/systemd"}

// PIDs returns the processes of the cgroup of a unit.
func PIDs(controlGroup string) []int {
	if controlGroup == "" {
		return nil
	}
	for _, root := range cgroupRoots {
		b, err := os.ReadFile(filepath.Join(root, controlGroup, "cgroup.procs"))
		if err != nil {
			continue
		}
		var l []int
		for _, s := range strings.Fields(string(b)) {
			if pid, err := strconv.Atoi(s); err == nil {
				l = append(l, pid)
			}
		}
		return l
	}
	return nil
}

// AnyPID returns a process of the unit, its main one if it has one, whose
// namespaces are the ones of the unit, false when it runs none.
func AnyPID(ctx context.Context, unit string) (int, bool, error) {
	st, err := GetState(ctx, unit)
	if err != nil {
		return 0, false, err
	}
	if !st.IsActive() {
		return 0, false, nil
	}
	if st.MainPID > 0 {
		return st.MainPID, true, nil
	}
	if l := PIDs(st.ControlGroup); len(l) > 0 {
		return l[0], true, nil
	}
	return 0, false, nil
}

// NsenterArgs returns the command entering the namespaces of the process
// pid, the mount, network, ipc and uts ones a unit sandboxes its processes
// in, to run what follows: a command sees then the files of the unit, as the
// ones of its private /tmp.
func NsenterArgs(pid int) []string {
	return []string{"nsenter", "--target", strconv.Itoa(pid), "--mount", "--net", "--ipc", "--uts", "--"}
}

// Kill sends sig to every process of the unit, which is no job.
func Kill(ctx context.Context, unit string, sig syscall.Signal) error {
	_, err := systemctl(ctx, "kill", "--signal="+strconv.Itoa(int(sig)), unit)
	return err
}

// ResetFailed forgets a unit that failed, which a unit of the same name
// starting again needs. A unit not loaded is no error.
func ResetFailed(ctx context.Context, unit string) error {
	st, err := GetState(ctx, unit)
	if err != nil {
		return err
	}
	if st.ActiveState != "failed" {
		return nil
	}
	_, err = systemctl(ctx, "reset-failed", unit)
	return err
}

// WaitInactive waits for the unit to run no process, up to timeout, and
// says whether it does.
func WaitInactive(ctx context.Context, unit string, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		st, err := GetState(ctx, unit)
		if err != nil {
			return false, err
		}
		if !st.IsActive() {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// Logger is what Terminate logs to.
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
}

// Terminate ends the processes of the unit still running, with a SIGTERM,
// then a SIGKILL to those still running after timeout, and forgets the unit
// if it failed, so a unit of the name can start again. No stop job is
// queued.
func Terminate(ctx context.Context, log Logger, unit string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	st, err := GetState(ctx, unit)
	if err != nil {
		return err
	}
	if st.IsActive() {
		log.Infof("send termination signal to the processes of the unit %s", unit)
		if err := Kill(ctx, unit, syscall.SIGTERM); err != nil {
			return err
		}
		if ok, err := WaitInactive(ctx, unit, timeout); err != nil {
			return err
		} else if !ok {
			log.Warnf("send kill signal to the processes of the unit %s, still running after %s", unit, timeout)
			if err := Kill(ctx, unit, syscall.SIGKILL); err != nil {
				return err
			}
			if ok, err := WaitInactive(ctx, unit, 5*time.Second); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("the unit %s still runs processes after a kill signal", unit)
			}
		}
	}
	return ResetFailed(ctx, unit)
}

// Journal returns the last n lines the invocation of a unit logged, as a
// command failing to start in it says why there.
func Journal(ctx context.Context, invocationID string, n int) []string {
	if invocationID == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "journalctl", "_SYSTEMD_INVOCATION_ID="+invocationID, "-o", "cat", "-n", strconv.Itoa(n), "--no-pager", "-q")
	b, err := cmd.Output()
	if err != nil {
		return nil
	}
	return slices.DeleteFunc(strings.Split(strings.TrimSpace(string(b)), "\n"), func(s string) bool { return s == "" })
}
