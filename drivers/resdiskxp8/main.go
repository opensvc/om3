package resdiskxp8

// Package resdiskxp8 implements the disk.xp8 driver for Hitachi/HPE XP8
// storage array replicated disk pairs.
//
// Each resource manages one or more LDEV pairs identified by their group
// name in the HORCM instance.

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/rs/zerolog"
)

const (
	// XP8 pair states returned by pairdisplay
	pairStatePAIR = "PAIR"
	pairStateCOPY = "COPY"
	pairStatePSUS = "PSUS"
	pairStatePSUE = "PSUE"
	pairStateSSUS = "SSUS"
	pairStateSSUE = "PSUE"
	pairStateSSWS = "SSWS"
	pairStateSYNC = "SYNC"
	pairStatePDUB = "PDUB"

	fenceNEVER  = "NEVER"
	fenceDATA   = "DATA"
	fenceSTATUS = "STATUS"

	roleSVOL = "S-VOL"
	rolePVOL = "P-VOL"
	roleSMPL = "SMPL"
)

// T is the driver structure embedding the common disk resource base.
type T struct {
	resdisk.T

	Path naming.Path `json:"path"`

	// Instance is the local HORCM instance number (used with -g and -I flags).
	Instance int `json:"instance"`

	// Group is the volume group name as defined in horcm.conf.
	Group string `json:"group"`

	// Timeout is the maximum duration to wait for a pair query operation to complete.
	Timeout *time.Duration `json:"timeout"`

	// StartTimeout is the maximum duration to wait for a pair takeover operation to complete.
	StartTimeout *time.Duration `json:"start_timeout"`

	// SplitStart must be set to allow start on a SSUS S-VOL (split pair)
	SplitStart bool `json:"split_start"`

	pairStatusCache *xpPairStatus
}

// pairdisplayLine represents one parsed line of pairdisplay -g <group> -l output.
type pairdisplayLine struct {
	Group      string
	Volume     string
	Local      bool
	DeviceFile string
	LDEV       string
	Role       string
	State      string
	Fence      string
	Copied     string
	M          string
}

var (
	ErrReplicationLinkFailed = errors.New("replication link failed")
)

// xpPairStatus holds the aggregated status of all pairs in the group.
type xpPairStatus struct {
	Lines     []pairdisplayLine
	statusMap map[string]any
	roleMap   map[string]any
	fenceMap  map[string]any
}

// New returns a new driver instance satisfying resource.Driver.
func New() resource.Driver {
	return &T{}
}

func (t *T) Configure() error {
	log := t.Log().AddPrefix(t.Name() + ": ")
	t.SetLoggerForTest(log)
	return nil
}

// Label returns a short human-readable description of the resource.
func (t *T) Label(_ context.Context) string {
	return fmt.Sprintf("horcm%d/%s", t.Instance, t.Group)
}

func (t *T) Name() string {
	return fmt.Sprintf("%d/%s", t.Instance, t.Group)
}

// Info returns key/value pairs used for resource info display.
func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := make(resource.InfoKeys, 0)
	m = append(m,
		resource.InfoKey{Key: "group", Value: t.Group},
		resource.InfoKey{Key: "instance", Value: fmt.Sprintf("%d", t.Instance)},
		resource.InfoKey{Key: "split_start", Value: fmt.Sprintf("%v", t.SplitStart)},
	)
	return m, nil
}

// Status returns the resource status.
//
// The resource is considered Up (in-sync) when all pairs report PAIR state.
// COPY mean a resync is in progress — reported as Warn.
// PSUS/PDUB are reported as Down.
func (t *T) Status(ctx context.Context) status.T {
	ps, err := t.cachedPairStatus(ctx)
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.NotApplicable
	}
	if len(ps.Lines) == 0 {
		t.StatusLog().Info("no volume pairs")
		return status.NotApplicable
	}

	for _, l := range ps.Lines {
		fmt := "pair volume:%s role:%s state:%s copied:%s"
		args := []any{l.Volume, l.Role, l.State, l.Copied + "%"}
		if l.Role == roleSMPL {
			t.StatusLog().Info(fmt, args...)
		} else {
			switch l.State {
			case pairStatePAIR:
				t.StatusLog().Info(fmt, args...)
			case pairStateCOPY, pairStateSYNC, pairStateSSUS, pairStatePSUS:
				t.StatusLog().Warn(fmt, args...)
			case "-":
				t.StatusLog().Warn(fmt, args...)
			default:
				t.StatusLog().Error(fmt, args...)
			}
		}
	}
	return status.NotApplicable
}

func (t *T) Boot(ctx context.Context) error {
	return t.testOrStartDaemon(ctx)
}

// Provisioned returns whether the XP8 pairs exist.
func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	ps, err := t.cachedPairStatus(ctx)
	if err != nil {
		return provisioned.False, err
	}
	if len(ps.Lines) == 0 {
		return provisioned.False, nil
	}
	return provisioned.True, nil
}

// SyncResync re-establishes the replication after a split.
func (t *T) Resync(ctx context.Context) error {
	ps, err := t.cachedPairStatus(ctx)
	if err != nil {
		return err
	}
	needResync := false
	for _, l := range ps.Lines {
		if l.State != pairStatePAIR {
			needResync = true
			break
		}
	}
	if !needResync {
		t.Log().Infof("already in PAIR state, nothing to do")
		return nil
	}
	return t.resync(ctx)
}

// Abort prevents starting the instance when we can forsee it will fail at this resource.
func (t *T) Abort(ctx context.Context) bool {
	ps, err := t.cachedPairStatus(ctx)
	if err != nil {
		t.Log().Warnf("abort? %s", err)
		return false
	}
	role := ps.Role()
	state := ps.Status()
	switch role {
	case roleSMPL:
	case rolePVOL:
		switch state {
		case pairStatePAIR, pairStatePSUS, pairStatePSUE:
		default:
			t.Log().Infof("abort! %s role is %s and state is unexpected %s, you have to manually return to a sane state.", t.Name(), role, state)
			return true
		}
	case roleSVOL:
		switch state {
		case pairStateCOPY:
		case pairStateSSUS, pairStateSSUE, pairStateSSWS:
			if !t.SplitStart {
				t.Log().Infof("abort! %s role is %s and state is %s, set the %s.split_start=true keyword if you really want to start even if the replication is suspended (the datasets will diverge and one will need to be dropped at some point)", t.Name(), role, state, t.RID())
				return true
			}
		case pairStatePAIR:
		default:
			t.Log().Infof("abort! %s role is S-VOL and state is unexpected %s , you have to manually return to a sane state.", t.Name(), state)
			return true
		}
	default:
		t.Log().Infof("abort! invalid role: %s", role)
		return true
	}
	return false
}

func (t *T) Stop(ctx context.Context) error {
	if err := t.testOrStartDaemon(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) Start(ctx context.Context) error {
	if err := t.testOrStartDaemon(ctx); err != nil {
		return err
	}
	ps, err := t.cachedPairStatus(ctx)
	if err != nil {
		return err
	}
	role := ps.Role()
	state := ps.Status()
	copied := ps.MinCopiedString()
	t.Log().Infof("role:%s state:%s copied:%s", role, state, copied)
	switch role {
	case roleSMPL:
	case rolePVOL:
		switch state {
		case pairStatePAIR, pairStatePSUS, pairStatePSUE:
			t.Log().Infof("assume already writable")
		default:
			return fmt.Errorf("unexpected %s:%s state, you have to manually return to a sane state", role, state)
		}
	case roleSVOL:
		switch state {
		case pairStatePAIR:
			err = t.failover(ctx, ps)
		case pairStateCOPY:
			if err := t.waitForState(ctx, pairStatePAIR); err != nil {
				return err
			}
			err = t.failover(ctx, ps)
		case pairStateSSUS, pairStateSSUE, pairStateSSWS:
			if !t.SplitStart {
				return fmt.Errorf("set the %s.split_start=true keyword if you really want to start even if the replication is suspended (the datasets will diverge and one will need to be dropped at some point)", t.RID())
			}
			t.Log().Infof("assume already writable")
		default:
			return fmt.Errorf("unexpected %s:%s state, you have to manually return to a sane state", role, state)
		}
	default:
		return fmt.Errorf("invalid role: %s", role)
	}
	if err != nil {
		return err
	}
	return t.promoteRW(ctx, ps)
}

func (t *T) failover(ctx context.Context, ps *xpPairStatus) error {
	exitCode, err := t.takeover(ctx)
	if err != nil {
		return err
	}

	if exitCode == 0 {
		return t.waitForState(ctx, pairStatePAIR)
	}
	if exitCode == 225 && ps.Fence() == fenceNEVER && ps.Role() == roleSVOL {
		t.Log().Debugf("make sure we left the COPY and PAIR state")
		if err := t.waitForNotState(ctx, pairStateCOPY, pairStatePAIR); err != nil {
			return err
		}
		ps, err = t.cachedPairStatus(ctx)
		if err != nil {
			return err
		}
		state := ps.Status()
		if state == pairStateSSWS {
			if err := t.resyncSwaps(ctx); err != nil {
				return err
			}
			return t.waitForState(ctx, pairStatePAIR)
		}
		return fmt.Errorf("the takeover failed with a pair end state %s we have no fallback plan for, you have to manually return to a sane state", state)
	}
	return nil
}

// SyncSplit splits the pair (suspend replication), making the R-VOL
// read-write accessible on the remote side.
func (t *T) Split(ctx context.Context) error {
	ps, err := t.cachedPairStatus(ctx)
	if err != nil {
		return err
	}
	needSplit := false
	for _, l := range ps.Lines {
		if l.State != pairStatePSUS {
			needSplit = true
			break
		}
	}
	if !needSplit {
		t.Log().Infof("already in PSUS state, nothing to do")
		return nil
	}
	return t.split(ctx)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (t *T) startTimeoutArg() string {
	return fmt.Sprintf("%d", int(t.StartTimeout.Seconds()))
}

func (t *T) takeover(ctx context.Context) (int, error) {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("horctakeover"),
		command.WithVarArgs("-g", t.Group, "-I"+t.instanceString()),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithIgnoredExitCodes(0, 1, 2, 3, 4, 5, 225),
	)
	err := cmd.Run()
	if err != nil {
		if b := cmd.Stdout(); len(b) > 0 {
			t.Log().Infof(string(b))
		}
		if b := cmd.Stderr(); len(b) > 0 {
			t.Log().Errorf(string(b))
		}
	} else {
		if b := cmd.Stdout(); len(b) > 0 {
			t.Log().Infof(string(b))
		}
		if b := cmd.Stderr(); len(b) > 0 {
			t.Log().Infof(string(b))
		}
	}
	exitCode := cmd.ExitCode()
	return exitCode, err
}

func (t *T) testOrStartDaemon(ctx context.Context) error {
	if err := t.testDaemon(ctx); err == nil {
		return nil
	}
	if err := t.startDaemon(ctx); err == nil {
		return nil
	}
	if err := t.testDaemon(ctx); err == nil {
		return nil
	}
	return fmt.Errorf("the horcm daemon has been started but horcctl still fails")
}

func (t *T) split(ctx context.Context) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("pairsplit"),
		command.WithVarArgs("-g", t.Group, "-I"+t.instanceString(), "-rw"),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return t.waitForState(ctx, pairStatePSUS)
}

func (t *T) splitSimplex(ctx context.Context) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("pairsplit"),
		command.WithVarArgs("-g", t.Group, "-I"+t.instanceString(), "-S"),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return t.waitForState(ctx, pairStatePSUS)
}

func (t *T) splitRW(ctx context.Context) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("pairsplit"),
		command.WithVarArgs("-g", t.Group, "-I"+t.instanceString(), "-rw"),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return t.waitForState(ctx, pairStatePSUS)
}

func (t *T) resync(ctx context.Context) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("pairresync"),
		command.WithVarArgs("-g", t.Group, "-I"+t.instanceString(), "-l"),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return t.waitForState(ctx, pairStatePAIR)
}

func (t *T) resyncSwaps(ctx context.Context) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("pairresync"),
		command.WithVarArgs("-g", t.Group, "-I"+t.instanceString(), "-swaps"),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithIgnoredExitCodes(0, 1, 2, 3, 4, 5),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return t.waitForState(ctx, pairStatePAIR)
}

func (t *T) testDaemon(ctx context.Context) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("horcctl"),
		command.WithVarArgs("-g", t.Group, "-I"+t.instanceString(), "-ND"),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
	)
	return cmd.Run()
}

func (t *T) startDaemon(ctx context.Context) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("horcmstart.sh"),
		command.WithVarArgs(fmt.Sprint(t.Instance)),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithIgnoredExitCodes(0, 1),
	)
	return cmd.Run()
}

func (t *T) stopDaemon(ctx context.Context) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("horcmshutdown.sh"),
		command.WithVarArgs(fmt.Sprint(t.Instance)),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithIgnoredExitCodes(0, 1),
	)
	return cmd.Run()
}

// pairStatus runs pairdisplay and parses the output.
func (t *T) pairStatus(ctx context.Context) (*xpPairStatus, error) {
	args := []string{
		"-g", t.Group,
		"-I" + fmt.Sprintf("%d", t.Instance),
		"-l",   // local display
		"-fcd", // include copy pct
		"-CLI", // machine-parseable output
	}
	out, err := t.runCmdOutput(ctx, "pairdisplay", args...)
	if err != nil {
		return nil, fmt.Errorf("pairdisplay failed: %w", err)
	}
	t.pairStatusCache = parsePairdisplay(out)
	return t.pairStatusCache, nil
}

func (t *T) cachedPairStatus(ctx context.Context) (*xpPairStatus, error) {
	if t.pairStatusCache != nil {
		return t.pairStatusCache, nil
	}
	return t.pairStatus(ctx)
}

func (t *T) pairEvWait(ctx context.Context) error {
	args := []string{
		"-g", t.Group,
		"-I" + fmt.Sprintf("%d", t.Instance),
		"-nowait",
	}
	cmd := exec.CommandContext(ctx, "pairevwait", args...)
	out, err := cmd.Output()
	exitCode := cmd.ProcessState.ExitCode()
	if err != nil {
		switch exitCode {
		case 5:
			return ErrReplicationLinkFailed
		case 3:
			// PAIR
			t.Log().Debugf("%s", out)
			return nil
		default:
			return fmt.Errorf("pairevwait failed: %w", err)
		}
	}
	return fmt.Errorf("unsupported link state: [%d] %s", exitCode, out)
}

// parsePairdisplay parses the -fcd columnar output of pairdisplay.
func parsePairdisplay(out string) *xpPairStatus {
	ps := &xpPairStatus{
		statusMap: make(map[string]any),
		roleMap:   make(map[string]any),
		fenceMap:  make(map[string]any),
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) < 12 {
			continue
		}
		if fields[1] == "PairVol" {
			// header
			continue
		}
		l := pairdisplayLine{
			Group:      fields[0],
			Volume:     fields[1],
			DeviceFile: fields[3],
			LDEV:       fields[5],
			Role:       fields[6],
			State:      fields[7],
			Fence:      fields[8],
			Copied:     fields[9],
			M:          fields[11],
		}
		if fields[2] == "L" {
			l.Local = true
		}
		ps.Lines = append(ps.Lines, l)
		ps.statusMap[l.State] = nil
		ps.roleMap[l.Role] = nil
		ps.fenceMap[l.Fence] = nil
	}
	return ps
}

func (t *xpPairStatus) IsSSUSWritable() bool {
	return true
}

func (t *xpPairStatus) MinCopiedString() string {
	return fmt.Sprint(t.MinCopied()) + "%"
}

func (t *xpPairStatus) MinCopied() int {
	i := 100
	for _, line := range t.Lines {
		copied, err := strconv.Atoi(line.Copied)
		if err != nil {
			return -1
		}
		if copied < i {
			i = copied
		}
	}
	return i
}

func (t *xpPairStatus) DeviceFiles(local bool) []string {
	m := make(map[string]any)
	for _, line := range t.Lines {
		if !local && line.Local {
			continue
		}
		if local && !line.Local {
			continue
		}
		m[line.DeviceFile] = nil
	}
	l := make([]string, len(m))
	i := 0
	for k := range m {
		l[i] = k
		i++
	}
	slices.Sort(l)
	return l
}

func (t *xpPairStatus) Fence() string {
	return strings.Join(t.FenceSet(), ",")
}

func (t *xpPairStatus) FenceSet() []string {
	l := make([]string, len(t.fenceMap))
	i := 0
	for k := range t.fenceMap {
		l[i] = k
		i++
	}
	slices.Sort(l)
	return l
}

func (t *xpPairStatus) Role() string {
	return strings.Join(t.RoleSet(), ",")
}

func (t *xpPairStatus) RoleSet() []string {
	l := make([]string, len(t.roleMap))
	i := 0
	for k := range t.roleMap {
		l[i] = k
		i++
	}
	slices.Sort(l)
	return l
}

func (t *xpPairStatus) Status() string {
	return strings.Join(t.StatusSet(), ",")
}

func (t *xpPairStatus) StatusSet() []string {
	l := make([]string, len(t.statusMap))
	i := 0
	for k := range t.statusMap {
		l[i] = k
		i++
	}
	slices.Sort(l)
	return l
}

func (t *xpPairStatus) StatusAll(l ...string) bool {
	for k := range t.statusMap {
		if !slices.Contains(l, k) {
			return false
		}
	}
	return true
}

// waitForState polls pairdisplay until all pairs reach one of the target states
// or the timeout is exceeded.
func (t *T) waitForState(ctx context.Context, states ...string) error {
	stateSet := make(map[string]struct{}, len(states))
	for _, s := range states {
		stateSet[s] = struct{}{}
	}
	for {
		ps, err := t.pairStatus(ctx)
		if err != nil {
			return err
		}
		allReached := true
		for _, l := range ps.Lines {
			if _, ok := stateSet[l.State]; !ok {
				allReached = false
				t.Log().Debugf("%s: waiting for state %s: current state %s", l.Volume, states, l.State)
				break
			}
		}
		if allReached {
			t.Log().Debugf("all pair volumes reached state %s", states)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

func (t *T) waitForNotState(ctx context.Context, states ...string) error {
	stateSet := make(map[string]struct{}, len(states))
	for _, s := range states {
		stateSet[s] = struct{}{}
	}
	for {
		ps, err := t.pairStatus(ctx)
		if err != nil {
			return err
		}
		allReached := true
		for _, l := range ps.Lines {
			if _, ok := stateSet[l.State]; ok {
				allReached = false
				t.Log().Debugf("%s: waiting for state not %s: current state %s", l.Volume, states, l.State)
				break
			}
		}
		if allReached {
			t.Log().Debugf("all pair volumes reached state not %s", states)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

func (t *T) instanceString() string {
	return fmt.Sprintf("%d", t.Instance)
}

func (t *T) runCmd(ctx context.Context, name string, args ...string) error {
	_, err := t.runCmdOutput(ctx, name, args...)
	return err
}

func (t *T) runCmdOutput(ctx context.Context, name string, args ...string) (string, error) {
	t.Log().Debugf("exec %s %s", name, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}
	t.Log().Debugf("%s", out)
	return string(out), nil
}

func (t *T) SubDevices(ctx context.Context) device.L {
	ps, err := t.cachedPairStatus(ctx)
	if err != nil {
		t.Log().Tracef("SubDevices: pairStatus: %s", err)
		return device.L{}
	}
	l, err := t.devices(ctx, ps)
	if err != nil {
		t.Log().Tracef("SubDevices: devices: %s", err)
		return device.L{}
	}
	return l
}

func (t *T) devices(ctx context.Context, ps *xpPairStatus) (device.L, error) {
	l := make(device.L, 0)
	for _, s := range ps.DeviceFiles(true) {
		dev := device.New("/dev/"+s, device.WithLogger(t.Log()))
		mpathDev, err := dev.MultipathParent()
		if err != nil {
			return l, err
		}
		if mpathDev != nil {
			l = append(l, *mpathDev)
		} else {
			l = append(l, dev)
		}
	}
	return l, nil
}

func (t *T) promoteRW(ctx context.Context, ps *xpPairStatus) error {
	devs, err := t.devices(ctx, ps)
	if err != nil {
		return err
	}
	t.Log().Tracef("devices to promote rw: %s", devs)
	for _, dev := range devs {
		if err := dev.PromoteRW(ctx); err != nil {
			return err
		}
	}
	return nil
}
