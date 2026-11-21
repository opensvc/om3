//go:build !windows

package resapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/sdunit"
	"github.com/opensvc/om3/v3/util/systemd"
)

// hasSystemd says the node runs systemd, replaced by the tests.
var hasSystemd = systemd.HasSystemd

// UnitMode says the start command of the resource runs in a transient
// systemd service unit, sandboxed by the properties systemd_properties sets.
func (t *T) UnitMode() bool {
	return strings.TrimSpace(t.SystemdProperties) != ""
}

// UnitName returns the name of the service unit the start command runs in:
// the one of the process group of the resource, as a service, in which it
// is placed, else one derived from the path of the object and the resource.
func (t *T) UnitName() string {
	if slice := t.unitSlice(); slice != "" {
		return strings.TrimSuffix(slice, ".slice") + ".service"
	}
	return "opensvc-" + systemd.Escape(strings.ReplaceAll(t.Path.String(), "/", ".")) + "-" + systemd.Escape(strings.ReplaceAll(t.RID(), "#", ".")) + ".service"
}

// unitSlice returns the slice of the process group of the resource, empty
// when it has none.
func (t *T) unitSlice() string {
	pg := t.GetPG()
	if pg == nil || pg.ID == "" {
		return ""
	}
	name := path.Base(pg.ID)
	if !strings.HasSuffix(name, ".slice") {
		return ""
	}
	return name
}

// unitEnvFile is the file the environment of the command is given to the
// unit in, which only root reads, rather than as properties every user sees.
func (t *T) unitEnvFile() string {
	return filepath.Join(t.VarDir(), "unit.env")
}

// UnitRun returns the options of the systemd-run command running the
// command s of the action in the unit of the resource, as r says, and the
// function removing the environment file once the unit read it.
//
// The command is om exec, with the user, the limits and the privileges of
// the resource, but no process group, the unit being placed in it. The
// environment is written in a file root alone reads. The process group is
// created with its cappings, which apply to the units in it.
func (t *T) UnitRun(ctx context.Context, s, action string, r sdunit.Run) ([]funcopt.O, func(), error) {
	cleanup := func() {}
	if !hasSystemd() {
		return nil, cleanup, fmt.Errorf("systemd_properties: the node does not run systemd")
	}
	props, err := sdunit.Parse(t.SystemdProperties)
	if err != nil {
		return nil, cleanup, fmt.Errorf("systemd_properties: %w", err)
	}
	if err := sdunit.Validate(t.SystemdProperties); err != nil {
		return nil, cleanup, fmt.Errorf("systemd_properties: %w", err)
	}
	cmdArgs, cred, err := t.CmdArgs(ctx, s, action)
	if err != nil || cmdArgs == nil {
		return nil, cleanup, err
	}
	if r.Type == "notify" {
		// om exec tells the unit it started, last before the command
		// takes over.
		cmdArgs = slices.Insert(cmdArgs, 2, "--sd-notify")
	}
	env, err := t.getEnv(ctx, func(err error) {
		t.Log().Infof("prepare '%s' command: %s", action, err)
	})
	if err != nil {
		return nil, cleanup, err
	}
	env = append(cred.env(), env...)
	b, err := sdunit.EnvFile(env)
	if err != nil {
		return nil, cleanup, err
	}
	envFile := t.unitEnvFile()
	if err := os.MkdirAll(filepath.Dir(envFile), 0o755); err != nil {
		return nil, cleanup, err
	}
	if err := os.WriteFile(envFile, b, 0o600); err != nil {
		return nil, cleanup, err
	}
	cleanup = func() {
		if err := os.Remove(envFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Log().Warnf("remove %s: %s", envFile, err)
		}
	}
	if slice := t.unitSlice(); slice != "" {
		if _, err := t.GetPG().ApplyProc(0); err != nil {
			cleanup()
			return nil, func() {}, err
		}
		r.Slice = slice
	}
	r.Unit = t.UnitName()
	r.Description = t.Path.String() + " " + t.RID()
	r.Properties = props
	r.OwnProperties = append(r.OwnProperties,
		sdunit.Property{Key: "Before", Value: sdunit.ServerUnit},
		sdunit.Property{Key: "EnvironmentFile", Value: envFile},
	)
	if t.Cwd != "" {
		r.OwnProperties = append(r.OwnProperties, sdunit.Property{Key: "WorkingDirectory", Value: t.Cwd})
	}
	r.Argv = cmdArgs
	// A unit of the name that failed is forgotten, or it could not start
	// again.
	if err := sdunit.ResetFailed(ctx, r.Unit); err != nil {
		t.Log().Warnf("%s", err)
	}
	args := r.Args()
	return []funcopt.O{
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
	}, cleanup, nil
}

// UnitStartTimeout returns the TimeoutStartSec property of the unit: the
// start timeout of the resource, infinity when none is set, as a command
// started in no unit is waited for.
func (t *T) UnitStartTimeout() sdunit.Property {
	timeout := t.GetTimeout("start")
	if timeout <= 0 {
		return sdunit.Property{Key: "TimeoutStartSec", Value: "infinity"}
	}
	return sdunit.Property{Key: "TimeoutStartSec", Value: strconv.FormatInt(int64(timeout/time.Millisecond), 10) + "ms"}
}

// UnitFailure returns the error of a unit that failed to start: err, and
// what the unit logged, as om exec saying why it could not run the command.
func (t *T) UnitFailure(ctx context.Context, err error) error {
	unit := t.UnitName()
	st, serr := sdunit.GetState(ctx, unit)
	if serr == nil {
		if lines := sdunit.Journal(ctx, st.InvocationID, 10); len(lines) > 0 {
			err = fmt.Errorf("%w: %s", err, strings.Join(lines, "; "))
		}
	}
	if terr := sdunit.Terminate(ctx, t.Log(), unit, t.GetTimeout("stop")); terr != nil {
		t.Log().Warnf("%s", terr)
	}
	return fmt.Errorf("unit %s: %w", unit, err)
}

// UnitTerminate ends the processes the unit of the resource still runs,
// given the stop timeout to end, and forgets the unit if it failed. No stop
// job is queued.
func (t *T) UnitTerminate(ctx context.Context) error {
	if !t.UnitMode() || !hasSystemd() {
		return nil
	}
	return sdunit.Terminate(ctx, t.Log(), t.UnitName(), t.GetTimeout("stop"))
}

// UnitLogJournal logs what the invocation of the unit logged, as the output
// of a start command, which goes to the journal.
func (t *T) UnitLogJournal(ctx context.Context) {
	st, err := sdunit.GetState(ctx, t.UnitName())
	if err != nil {
		return
	}
	for _, line := range sdunit.Journal(ctx, st.InvocationID, 50) {
		t.Log().Infof("%s", line)
	}
}

// unitNsenter returns the command entering the namespaces of the unit of
// the resource, which a command other than the start one runs in, so it
// sees the files the command sees: empty when the unit runs no process.
func (t *T) unitNsenter(ctx context.Context, action string) []string {
	if !t.UnitMode() || !hasSystemd() {
		return nil
	}
	switch action {
	case "start", "run":
		return nil
	}
	pid, ok, err := sdunit.AnyPID(ctx, t.UnitName())
	if err != nil {
		t.Log().Debugf("%s", err)
		return nil
	}
	if !ok {
		return nil
	}
	return sdunit.NsenterArgs(pid)
}
