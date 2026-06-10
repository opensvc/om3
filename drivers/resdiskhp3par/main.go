package resdiskhp3par

// Package resdiskhp3par implements the disk.hp3par driver for HPE 3PAR
// storage array replicated disk volumes.
//
// Each resource manages one remote copy group (RCG) for volume replication
// between HPE 3PAR storage arrays.
//
// Configuration for the array connection is read from the cluster.conf
// or node.conf array#<suffix> section, where <suffix> is the value of the
// array keyword.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/ageingcache"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/duration"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/rs/zerolog"
)

const (
	// Remote copy group states
	groupStatusStarted  = "Started"
	groupStatusStopped  = "Stopped"
	groupStatusStarting = "Starting"
	groupStatusStopping = "Stopping"

	// Remote copy group roles
	groupRolePrimary      = "Primary"
	groupRolePrimaryRev   = "Primary-Rev"
	groupRoleSecondary    = "Secondary"
	groupRoleSecondaryRev = "Secondary-Rev"

	// Remote copy group modes
	groupModeSync     = "Sync"
	groupModePeriodic = "Periodic"

	// Remote copy group options
	groupOptionAutoRecover = "auto_recover"

	// Target status
	targetStatusFailed  = "failed"
	targetStatusFailing = "failing"
	targetStatusReady   = "ready"

	// Volume sync status
	vvSyncStatusNew       = "New"
	vvSyncStatusNotSynced = "NotSynced"
	vvSyncStatusStale     = "Stale"
	vvSyncStatusStopped   = "Stopped"
	vvSyncStatusSynced    = "Synced"
	vvSyncStatusSyncing   = "Syncing"

	// Volume last sync time (a valid date for async or "NA")
	vvLastSyncTimeNA = "NA"

	// Command method
	methodSSH = "ssh"
	methodCLI = "cli"

	lockName = "hp3par"
)

// Config holds the array configuration from the cluster config.
type Config struct {
	// Method is the connection method: ssh or cli
	Method string `json:"method"`
	// Manager is the array name or IP address
	Manager string `json:"manager"`
	// Username for SSH connections
	Username string `json:"username,omitempty"`
	// Key is the SSH private key file path or datastore reference
	Key string `json:"key,omitempty"`
	// CLI is the 3PAR CLI binary path or datastore reference
	CLI string `json:"cli,omitempty"`
	// PWF is the password file for CLI connections or datastore reference
	PWF string `json:"pwf,omitempty"`
}

// T is the driver structure embedding the common disk resource base.
type T struct {
	resdisk.T

	Path naming.Path `json:"path"`

	// Array is the suffix of the array configuration section (array#<suffix>).
	Array string `json:"array"`

	// Group is the name of the HP 3PAR remote copy group.
	Group string `json:"group"`

	// Mode is the replication mode: "sync" or "async".
	Mode string `json:"mode"`

	// MaxDelay is the max age of the last sync.
	MaxDelay *time.Duration `json:"max_delay"`

	// ForceSync trigger a sync before migrate if the group mode is Periodic.
	ForceSync bool

	// AutoTakeover allows failover when the arrays are split.
	AutoTakeover bool

	// Allow role swap when the arrays are joined.
	SwapRoles bool

	// Timeout is the maximum duration to wait for operations to complete.
	Timeout *time.Duration `json:"timeout"`

	// StartTimeout is the maximum duration to wait for start operations.
	StartTimeout *time.Duration `json:"start_timeout"`

	// arrayConfig holds the resolved array configurations
	arrayConfig map[string]*Config

	// Cached resolved values for datastore-referenced files
	keyFileCache string
	pwfCache     string

	groupStatus  *groupStatus
	targetStatus *targetStatus
}

// groupStatus holds the status information for a remote copy group.
type groupStatus struct {
	Name    string
	Target  string
	Status  string
	Role    string
	Mode    string
	Options []string
	Volumes []vvStatus
}

type targetStatus struct {
	Name    string
	ID      string
	Type    string
	Status  string
	Options string
	Policy  string
}

// vvStatus holds the status information for a volume in a remote copy group.
type vvStatus struct {
	LocalVV      string
	LocalVVID    string
	RemoteVV     string
	RemoteVVID   string
	SyncStatus   string
	LastSyncTime time.Time
}

var (
	ErrBuildCommand       = errors.New("error building command")
	ErrRCGNotFound        = errors.New("remote copy group not found")
	ErrArrayNotAccessible = errors.New("array not accessible")
	ErrReplicationFailed  = errors.New("replication operation failed")
)

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
	return t.Name()
}

func (t *T) Name() string {
	return fmt.Sprintf("%s/%s", t.Array, t.Group)
}

// Info returns key/value pairs used for resource info display.
func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := make(resource.InfoKeys, 0)
	m = append(m,
		resource.InfoKey{Key: "array", Value: t.Array},
		resource.InfoKey{Key: "group", Value: t.Group},
		resource.InfoKey{Key: "mode", Value: t.Mode},
		resource.InfoKey{Key: "force_sync", Value: fmt.Sprint(t.ForceSync)},
		resource.InfoKey{Key: "auto_takeover", Value: fmt.Sprint(t.AutoTakeover)},
		resource.InfoKey{Key: "swap_roles", Value: fmt.Sprint(t.SwapRoles)},
	)
	if t.Timeout != nil {
		m = append(m, resource.InfoKey{Key: "timeout", Value: fmt.Sprintf("%s", t.Timeout)})
	}
	if t.StartTimeout != nil {
		m = append(m, resource.InfoKey{Key: "start_timeout", Value: fmt.Sprintf("%s", t.StartTimeout)})
	}
	return m, nil
}

// Status returns the resource status.
func (t *T) Status(ctx context.Context) status.T {
	if err := t.initStatus(ctx); err != nil {
		t.StatusLog().Error("%s", err)
		return status.NotApplicable
	}

	if t.groupStatus == nil {
		t.StatusLog().Info("no group status available")
		return status.NotApplicable
	}

	t.StatusLog().Info(t.groupStatus.String())

	// Check overall RCG status
	if t.groupStatus.Status != groupStatusStarted {
		t.StatusLog().Warn("state should be %s", groupStatusStarted)
	}

	// Check role based on mode
	if t.Mode == "sync" {
		if t.groupStatus.Mode != groupModeSync {
			t.StatusLog().Warn("mode should be %s", groupModeSync)
		}
	} else if t.Mode == "async" {
		if t.groupStatus.Mode != groupModePeriodic {
			t.StatusLog().Warn("mode should be %s", groupModePeriodic)
		}
	}

	// Check volume sync status
	period, err := t.groupStatus.Period()
	if err != nil {
		t.StatusLog().Error("%s", err)
	} else if period > 0 {
		maxDelay := t.getMaxDelay(period)
		for _, vv := range t.groupStatus.Volumes {
			if vv.SyncStatus != vvSyncStatusSynced {
				t.StatusLog().Warn("volume %s sync status is %s (expected Synced)", vv.LocalVV, vv.SyncStatus)
			}
			cutoff := time.Now().UTC().Add(-1 * maxDelay)
			if vv.LastSyncTime.Before(cutoff) {
				t.StatusLog().Warn("volume %s last sync too old (%s, over %s)", vv.LocalVV, vv.LastSyncTime.Format("2006-01-02 15:04:05"), duration.FmtShortDuration(maxDelay))
			}
		}
	}

	return status.NotApplicable
}

// Provisioned returns whether the RCG exists.
func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	if err := t.initStatus(ctx); err != nil {
		return provisioned.False, err
	}
	if t.groupStatus == nil {
		return provisioned.False, nil
	}
	return provisioned.True, nil
}

// Start starts the replication in the appropriate direction.
func (t *T) Start(ctx context.Context) error {
	if err := t.initStatus(ctx); err != nil {
		return err
	}

	switch t.groupStatus.Role {
	case groupRolePrimary, groupRolePrimaryRev:
		t.Log().Infof("group role is already %s, skip", t.groupStatus.Role)
		if t.groupStatus.Status != groupStatusStarted {
			t.Log().Warnf("group status is %s", t.groupStatus.Status)
		}
		return t.promoteRW(ctx)
	}

	// Wait for target status to leave the failing state
	if err := t.waitValidTargetStatus(ctx, t.groupStatus.Target); err != nil {
		return err
	}

	if err := t.rsyncGroupRemote(ctx); err != nil {
		return err
	}

	switch t.targetStatus.Status {
	case targetStatusFailed:
		t.Log().Infof("we are split from %s array", t.groupStatus.Target)
		return t.failover(ctx)
	case targetStatusReady:
		t.Log().Infof("we are joined with %s array", t.groupStatus.Target)
		return t.migrate(ctx)
	default:
		return fmt.Errorf("unsupported target status: %s", t.groupStatus.Target)
	}

	return t.promoteRW(ctx)
}

func (t *T) getMaxDelay(period time.Duration) time.Duration {
	if t.MaxDelay != nil && *t.MaxDelay > 0 {
		return *t.MaxDelay
	}
	return period * 2
}

func (t *T) rsyncGroupRemote(ctx context.Context) error {
	syncStatus := t.groupStatus.SyncStatus()
	switch syncStatus {
	case vvSyncStatusNew, vvSyncStatusNotSynced:
		return fmt.Errorf("sync status is %s: skip rcopygroup", syncStatus)
	case vvSyncStatusStale:
		if t.targetStatus.Status == targetStatusFailed {
			return fmt.Errorf("sync status is %s and target is %s: skip rcopygroup", syncStatus, targetStatusFailed)
		}
	case vvSyncStatusSyncing:
		t.Log().Infof("state is syncing, wait")
		if err := t.waitRCGStatusSync(ctx); err != nil {
			return err
		}
	}

	switch t.groupStatus.Status {
	case groupStatusStarted:
	case groupStatusStopped:
		if t.targetStatus.Status != targetStatusFailed {
			return fmt.Errorf("group replication is stopped but target is not failed. block start to avoid making a delta on both sites. an administrator needs to discard one dataset and restart the replication.")
		}
	}

	switch t.targetStatus.Status {
	case targetStatusReady, targetStatusFailed:
	default:
		return fmt.Errorf("target status is %s", t.targetStatus.Status)
	}

	if t.ForceSync && t.groupStatus.Mode == groupModePeriodic && t.groupStatus.SyncStatus() == vvSyncStatusSynced {
		return t.remoteSyncUpdate(ctx)
	}
	return nil
}

func (t *T) remoteSyncUpdate(ctx context.Context) error {
	if err := t.syncArrayRCG(ctx, t.groupStatus.Target); err != nil {
		return err
	}
	if err := t.waitRCGStatusSync(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) migrate(ctx context.Context) error {
	defer func() { t.refreshGroupStatus(ctx) }()
	if t.SwapRoles {
		if err := t.reverseStopgroup(ctx); err != nil {
			return err
		}
		if err := t.runStartGroup(ctx); err != nil {
			return err
		}
	} else {
		if err := t.reverseStopgroupCurrent(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) failover(ctx context.Context) error {
	if t.groupStatus.Status != groupStatusStopped {
		return fmt.Errorf("group status is %s, expecting %s", t.groupStatus.Status, groupStatusStopped)
	}
	if syncStatus := t.groupStatus.SyncStatus(); syncStatus != vvSyncStatusStopped {
		return fmt.Errorf("group sync status is %s, expecting %s", syncStatus, vvSyncStatusStopped)
	}
	if !t.AutoTakeover {
		t.Log().Infof("the 'auto_takeover' keyword value is false: skip failover")
		return nil
	}
	switch t.groupStatus.Role {
	case groupRoleSecondary:
		return t.failoverSecondary(ctx)
	case groupRoleSecondaryRev:
		return t.failoverSecondaryRev(ctx)
	}
	return nil
}

// Stop stops the replication.
func (t *T) Stop(ctx context.Context) error {
	return nil
}

// Resync re-establishes the replication after a split.
func (t *T) Resync(ctx context.Context) error {
	if err := t.initStatus(ctx); err != nil {
		return err
	}
	return t.syncRCG(ctx)
}

// Update performs a sync operation.
func (t *T) Update(ctx context.Context) error {
	if err := t.initStatus(ctx); err != nil {
		return err
	}
	return t.syncRCG(ctx)
}

// Swap swaps the replication direction.
func (t *T) Swap(ctx context.Context) error {
	if err := t.initStatus(ctx); err != nil {
		return err
	}
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	unlock, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer unlock()

	if t.groupStatus.Role == groupRolePrimary {
		return fmt.Errorf("rcopy group %s role is Primary, refuse to swap", t.Group)
	}
	if err := t.stopRCG(ctx); err != nil {
		return err
	}
	if err := t.setRCGReverse(ctx); err != nil {
		return err
	}
	if err := t.refreshGroupStatus(ctx); err != nil {
		return err
	}
	return t.startGroup(ctx)
}

// Resume resumes the replication.
func (t *T) Resume(ctx context.Context) error {
	if err := t.initStatus(ctx); err != nil {
		return err
	}
	return t.startGroup(ctx)
}

// Split quiesces the replication.
func (t *T) Split(ctx context.Context) error {
	if err := t.initStatus(ctx); err != nil {
		return err
	}
	return t.stopRCG(ctx)
}

// ---------------------------------------------------------------------------
// Array configuration loading
// ---------------------------------------------------------------------------

// loadArrayConfig loads the array configuration from the cluster config.
// Configuration is read from the array#<suffix> section where <suffix> is t.Array.
func (t *T) loadArrayConfig() error {
	return t.loadThisArrayConfig(t.Array)
}

func (t *T) loadThisArrayConfig(arrayName string) error {
	if _, ok := t.arrayConfig[arrayName]; ok {
		return nil
	}

	config := &Config{}

	node, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return err
	}

	cfg := node.MergedConfig()
	if cfg == nil {
		return fmt.Errorf("no node config available")
	}

	// Get the array section from cluster config
	sectionName := fmt.Sprintf("array#%s", arrayName)

	// Get method
	if v := cfg.GetString(key.T{Section: sectionName, Option: "method"}); v != "" {
		config.Method = v
	} else {
		return fmt.Errorf("method is required in array#%s configuration", t.Array)
	}

	// Get manager (array name/IP)
	if v := cfg.GetString(key.T{Section: sectionName, Option: "manager"}); v != "" {
		config.Manager = v
	} else {
		// If manager is not set, use the array suffix as the manager name
		config.Manager = t.Array
	}

	// Get username for SSH
	if v := cfg.GetString(key.T{Section: sectionName, Option: "username"}); v != "" {
		config.Username = v
	}

	// Get key (SSH private key) - can be a file path or datastore reference
	if v := cfg.GetString(key.T{Section: sectionName, Option: "key"}); v != "" {
		config.Key = v
	}

	// Get cli (CLI binary path) - can be a file path or datastore reference
	if v := cfg.GetString(key.T{Section: sectionName, Option: "cli"}); v != "" {
		config.CLI = v
	} else {
		config.CLI = "cli" // default
	}

	// Get pwf (password file) - can be a file path or datastore reference
	if v := cfg.GetString(key.T{Section: sectionName, Option: "pwf"}); v != "" {
		config.PWF = v
	}

	if t.arrayConfig == nil {
		t.arrayConfig = make(map[string]*Config)
	}
	t.arrayConfig[arrayName] = config
	t.Log().Tracef("loaded array config: %#v", config)
	return nil
}

// getArrayConfig returns the loaded array configuration.
func (t *T) getArrayConfig(arrayName string) (*Config, error) {
	if err := t.loadThisArrayConfig(arrayName); err != nil {
		return nil, err
	}
	return t.arrayConfig[arrayName], nil
}

// ---------------------------------------------------------------------------
// Datastore-backed configuration resolution
// ---------------------------------------------------------------------------

// manager returns the resolved manager value from array config.
func (t *T) manager(arrayName string) (string, error) {
	config, err := t.getArrayConfig(arrayName)
	if err != nil {
		return "", err
	}
	return config.Manager, nil
}

// username returns the resolved username value from array config.
func (t *T) username(arrayName string) (string, error) {
	config, err := t.getArrayConfig(arrayName)
	if err != nil {
		return "", err
	}
	return config.Username, nil
}

// method returns the resolved method value from array config.
func (t *T) method(arrayName string) (string, error) {
	config, err := t.getArrayConfig(arrayName)
	if err != nil {
		return "", err
	}
	return config.Method, nil
}

// keyFile returns the resolved keyfile path, supporting "from <path> key <key>" format.
// The content is cached as a temporary file.
func (t *T) keyFile(arrayName string) (string, error) {
	config, err := t.getArrayConfig(arrayName)
	if err != nil {
		return "", err
	}
	if config.Key == "" {
		return "", nil
	}
	if strings.HasPrefix(config.Key, "/") {
		return config.Key, nil
	}
	if t.keyFileCache != "" {
		return t.keyFileCache, nil
	}
	km, err := datarecv.ParseKeyMetaRelObj(config.Key, t.GetObject())
	if err != nil {
		t.keyFileCache = ""
		return "", err
	}
	file, err := km.CacheFile()
	if err != nil {
		t.keyFileCache = ""
		return "", err
	}
	t.keyFileCache = file
	return file, nil
}

// pwf returns the resolved pwf path, supporting "from <path> key <key>" format.
// The content is cached as a temporary file.
func (t *T) pwf(arrayName string) (string, error) {
	config, err := t.getArrayConfig(arrayName)
	if err != nil {
		return "", err
	}
	if config.PWF == "" {
		return "", nil
	}
	if strings.HasPrefix(config.PWF, "/") {
		return config.PWF, nil
	}
	if t.pwfCache != "" {
		return t.pwfCache, nil
	}
	km, err := datarecv.ParseKeyMetaRelObj(config.PWF, t.GetObject())
	if err != nil {
		t.pwfCache = ""
		return "", err
	}
	file, err := km.CacheFile()
	if err != nil {
		t.pwfCache = ""
		return "", err
	}
	t.pwfCache = file
	return file, nil
}

// cli returns the resolved cli path.
// The content is cached as a temporary file.
func (t *T) cli(arrayName string) (string, error) {
	config, err := t.getArrayConfig(arrayName)
	if err != nil {
		return "", err
	}
	return config.CLI, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (t *T) timeoutArg() string {
	if t.Timeout != nil {
		return fmt.Sprintf("%d", int(t.Timeout.Seconds()))
	}
	return "10"
}

func (t *T) startTimeoutArg() string {
	if t.StartTimeout != nil {
		return fmt.Sprintf("%d", int(t.StartTimeout.Seconds()))
	}
	return "300"
}

func (t *T) buildSSHCommand(arrayName, cmd string) ([]string, error) {
	// For SSH method: ssh -i <key> <username>@<manager>
	keyFile, err := t.keyFile(arrayName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve keyfile: %w", err)
	}
	username, err := t.username(arrayName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve username: %w", err)
	}
	manager, err := t.manager(arrayName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve manager: %w", err)
	}

	args := []string{"ssh"}
	if keyFile != "" {
		args = append(args, "-i", keyFile)
	}
	if username != "" {
		args = append(args, username+"@"+manager)
	} else {
		args = append(args, manager)
	}
	args = append(args, cmd)
	return args, nil
}

func (t *T) buildCLICommand(arrayName, cmd string) ([]string, error) {
	// For CLI method: <cli> -sys <manager> -pwf <pwf> <command>
	cli, err := t.cli(arrayName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cli: %w", err)
	}
	manager, err := t.manager(arrayName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve manager: %w", err)
	}
	pwf, err := t.pwf(arrayName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve pwf: %w", err)
	}

	args := []string{cli, "-sys", manager}
	if pwf != "" {
		args = append(args, "-pwf", pwf)
	}
	args = append(args, strings.Fields(cmd)...)
	return args, nil
}

func (t *T) buildCommand(arrayName, cmd string) ([]string, error) {
	method, err := t.method(arrayName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve method: %w", err)
	}

	switch method {
	case methodSSH:
		return t.buildSSHCommand(arrayName, cmd)
	case methodCLI:
		return t.buildCLICommand(arrayName, cmd)
	default:
		return nil, fmt.Errorf("%w: unknown method %s", ErrBuildCommand, method)
	}
}

func (t *T) waitRCGStatusSync(ctx context.Context) error {
	interval := 5 * time.Second
	defaultTimeout := 2 * time.Minute

	// The context should have the StartTimeout, but don't risk entering a
	// infinite loop.
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultTimeout)
	}

	for {
		gs, err := t.getGroupStatus(ctx)
		if err != nil {
			return err
		}
		t.groupStatus = gs
		if gs.SyncStatus() == vvSyncStatusSyncing {
			t.Log().Infof("vv are still syncing, retry later")
			if deadline.Before(time.Now().Add(interval)) {
				return fmt.Errorf("timeout waiting for all vv to leave the 'syncing' status")
			}
			time.Sleep(interval)
			continue
		}
		return nil
	}
	return nil
}

func (t *T) waitValidTargetStatus(ctx context.Context, target string) error {
	interval := 5 * time.Second
	defaultTimeout := 2 * time.Minute

	// The context should have the StartTimeout, but risk entering a
	// infinite loop.
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultTimeout)
	}

	for {
		ts, err := t.getTargetStatus(ctx)
		if err != nil {
			return err
		}
		t.targetStatus = ts
		if t.targetStatus.Status == targetStatusFailing {
			t.Log().Infof("target failing, retry later")
			if deadline.Before(time.Now().Add(interval)) {
				return fmt.Errorf("timeout waiting for target to leave the 'failing' status")
			}
			time.Sleep(interval)
			continue
		}
		return nil
	}
	return nil
}

func (t *T) getGroupWWN(ctx context.Context) ([]string, error) {
	cmdS := fmt.Sprintf("showvv -showcols VV_WWN -p -type base -rcopygroup %s -csvtable -nohdtot", t.Group)
	cmdV, err := t.buildCommand(t.Array, cmdS)
	if err != nil {
		return nil, err
	}
	if len(cmdV) < 2 {
		return nil, fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cmdV[0]),
		command.WithArgs(cmdV[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
		command.WithStderrLogLevel(zerolog.TraceLevel),
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var wwns []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "6") {
			wwns = append(wwns, line)
		}
	}
	return wwns, nil
}

func (t *T) getTargetStatus(ctx context.Context) (*targetStatus, error) {
	cmdS := fmt.Sprintf("showrcopy targets -csvtable -nohdtot")
	cmdV, err := t.buildCommand(t.Array, cmdS)
	if err != nil {
		return nil, err
	}
	if len(cmdV) < 2 {
		return nil, fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cmdV[0]),
		command.WithArgs(cmdV[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
		command.WithStderrLogLevel(zerolog.TraceLevel),
	)
	var out []byte
	if actioncontext.Props(ctx).Name == actioncontext.Status.Name {
		sig := fmt.Sprintf("hp3par-%s-showrcopy-targets", t.Array)
		maxAge := 1 * time.Minute
		out, err = ageingcache.Output(cmd, sig, maxAge)
	} else {
		out, err = cmd.Output()
	}
	if err != nil {
		return nil, err
	}
	return t.parseTargetStatus(string(out))
}

func (t *T) getGroupStatus(ctx context.Context) (*groupStatus, error) {
	cmdS := fmt.Sprintf("showrcopy groups -csvtable -nohdtot")
	cmdV, err := t.buildCommand(t.Array, cmdS)
	if err != nil {
		return nil, err
	}
	if len(cmdV) < 2 {
		return nil, fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cmdV[0]),
		command.WithArgs(cmdV[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
		command.WithStderrLogLevel(zerolog.TraceLevel),
	)
	var out []byte
	if actioncontext.Props(ctx).Name == actioncontext.Status.Name {
		sig := fmt.Sprintf("hp3par-%s-showrcopy-groups", t.Array)
		maxAge := 1 * time.Minute
		out, err = ageingcache.Output(cmd, sig, maxAge)
	} else {
		out, err = cmd.Output()
	}
	if err != nil {
		return nil, err
	}
	return t.parseGroupStatus(string(out), t.Group)
}

func (t *T) initStatus(ctx context.Context) error {
	if gs, err := t.getGroupStatus(ctx); err != nil {
		return err
	} else {
		t.groupStatus = gs
	}
	if ts, err := t.getTargetStatus(ctx); err != nil {
		return err
	} else {
		t.targetStatus = ts
	}
	return nil
}

func (t *T) refreshGroupStatus(ctx context.Context) error {
	if gs, err := t.getGroupStatus(ctx); err != nil {
		return err
	} else {
		t.groupStatus = gs
	}
	return nil
}

func (t *T) refreshTargetStatus(ctx context.Context) error {
	if ts, err := t.getTargetStatus(ctx); err != nil {
		return err
	} else {
		t.targetStatus = ts
	}
	return nil
}

func (t *T) parseTargetStatus(out string) (*targetStatus, error) {
	// Format:
	// Name,ID,Type,Status,Options,Policy
	// baie-pra.cgr.fr,2,FC,ready,2FF70002AC00992B,mirror_config
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		fields := t.splitCSV(line)
		if len(fields) < 6 {
			return nil, fmt.Errorf("unexpected target status format: %s", line)
		}
		ts := targetStatus{
			Name:    fields[0],
			ID:      fields[1],
			Type:    fields[2],
			Status:  fields[3],
			Options: fields[4],
			Policy:  fields[5],
		}
		if ts.Name == t.groupStatus.Target {
			return &ts, nil
		}
	}
	return nil, fmt.Errorf("target not found")
}

func (t *T) parseGroupStatus(out string, groupName string) (*groupStatus, error) {
	// Parse the showrcopy groups output
	// Format:
	// Name,Target,Status,Role,Mode,Options
	// rcg1,target1,Started,Primary,Sync,"opt1,opt2"
	//  ,LocalVV,ID,RemoteVV,ID,SyncStatus,LastSyncTime
	//  ,vol1,1,vol2,2,Synced,2024-01-01 12:00:00

	lines := strings.Split(out, "\n")
	var gs *groupStatus
	var inRCGBlock bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is the start of our RCG block
		if strings.HasPrefix(line, groupName+",") {
			inRCGBlock = true
			ps := t.parseRCGLine(line)
			if ps != nil {
				gs = ps
				gs.Name = groupName
			}
			continue
		}

		if !inRCGBlock {
			continue
		}

		// Check if we've left the RCG block
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, ",") {
			break
		}

		// Parse volume line
		if strings.HasPrefix(line, ",") || strings.HasPrefix(line, " ") {
			vv := t.parseVVLine(line)
			if vv != nil && gs != nil {
				gs.Volumes = append(gs.Volumes, *vv)
			}
		}
	}

	if gs == nil {
		return nil, fmt.Errorf("group %s not found", groupName)
	}

	return gs, nil
}

func (t *T) parseRCGLine(line string) *groupStatus {
	// Parse line like: Name,Target,Status,Role,Mode,"Options"
	// Remove leading/trailing spaces and quotes
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, ",") {
		line = line[1:]
	}

	// Split by comma, respecting quoted strings
	parts := t.splitCSV(line)
	if len(parts) < 5 {
		return nil
	}

	gs := &groupStatus{
		Status: parts[2],
		Role:   parts[3],
		Mode:   parts[4],
	}

	// Parse options from the 6th field onwards
	if len(parts) >= 6 {
		optionsStr := strings.Join(parts[5:], ",")
		// Remove quotes and split by comma
		optionsStr = strings.Trim(optionsStr, `"`)
		for _, opt := range strings.Split(optionsStr, ",") {
			opt = strings.TrimSpace(opt)
			if opt != "" {
				gs.Options = append(gs.Options, opt)
			}
		}
	}

	if len(parts) >= 2 {
		gs.Target = parts[1]
	}

	return gs
}

func (t *T) parseVVLine(line string) *vvStatus {
	// Parse line like: ,LocalVV,ID,RemoteVV,ID,SyncStatus,LastSyncTime
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, ",") {
		line = line[1:]
	}

	parts := t.splitCSV(line)
	if len(parts) < 6 {
		return nil
	}

	vv := &vvStatus{
		LocalVV:    parts[0],
		LocalVVID:  parts[1],
		RemoteVV:   parts[2],
		RemoteVVID: parts[3],
		SyncStatus: parts[4],
	}

	// Parse LastSyncTime
	timeStr := strings.TrimSpace(parts[5])
	if timeStr != "" && timeStr != vvLastSyncTimeNA {
		// Try to parse the time string
		parsedTime, err := time.Parse("2006-01-02 15:04:05 MST", timeStr)
		if err == nil {
			vv.LastSyncTime = parsedTime.UTC()
		} else {
			// Try other formats
			parsedTime, err = time.Parse("2006-01-02 15:04:05", timeStr)
			if err == nil {
				vv.LastSyncTime = parsedTime.UTC()
			} else {
				t.Log().Warnf("unable to parse time: %s", timeStr)
			}
		}
	}

	return vv
}

func (t *T) splitCSV(line string) []string {
	// Simple CSV split that respects quotes
	var parts []string
	var current strings.Builder
	inQuotes := false

	for _, r := range line {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				parts = append(parts, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	parts = append(parts, strings.TrimSpace(current.String()))
	return parts
}

func (t *T) failoverSecondary(ctx context.Context) error {
	cmdS := fmt.Sprintf("setrcopygroup failover -f -waittask %s", t.Group)
	cmdV, err := t.buildCommand(t.Array, cmdS)
	if err != nil {
		return err
	}
	if len(cmdV) < 2 {
		return fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cmdV[0]),
		command.WithArgs(cmdV[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

func (t *T) failoverSecondaryRev(ctx context.Context) error {
	cmdS := fmt.Sprintf("setrcopygroup reverse -f -local -current -waittask %s", t.Group)
	cmdV, err := t.buildCommand(t.Array, cmdS)
	if err != nil {
		return err
	}
	if len(cmdV) < 2 {
		return fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cmdV[0]),
		command.WithArgs(cmdV[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

func (t *T) reverseStopgroup(ctx context.Context) error {
	cmdS := fmt.Sprintf("setrcopygroup reverse -f -stopgroups -waittask %s", t.Group)
	cmdV, err := t.buildCommand(t.Array, cmdS)
	if err != nil {
		return err
	}
	if len(cmdV) < 2 {
		return fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cmdV[0]),
		command.WithArgs(cmdV[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

func (t *T) reverseStopgroupCurrent(ctx context.Context) error {
	cmdS := fmt.Sprintf("setrcopygroup reverse -f -current -stopgroups -waittask %s", t.Group)
	cmdV, err := t.buildCommand(t.Array, cmdS)
	if err != nil {
		return err
	}
	if len(cmdV) < 2 {
		return fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cmdV[0]),
		command.WithArgs(cmdV[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

func (t *T) reverseStopgroupNatural(ctx context.Context) error {
	cmdS := fmt.Sprintf("setrcopygroup reverse -f -natural -stopgroups -waittask %s", t.Group)
	cmdV, err := t.buildCommand(t.Array, cmdS)
	if err != nil {
		return err
	}
	if len(cmdV) < 2 {
		return fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cmdV[0]),
		command.WithArgs(cmdV[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

func (t *T) reverse(ctx context.Context) error {
	cmdS := fmt.Sprintf("setrcopygroup reverse -f -waittask %s", t.Group)
	cmdV, err := t.buildCommand(t.Array, cmdS)
	if err != nil {
		return err
	}
	if len(cmdV) < 2 {
		return fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cmdV[0]),
		command.WithArgs(cmdV[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

func (t *T) runStartGroup(ctx context.Context) error {
	cmdS := fmt.Sprintf("startrcopygroup -wait %s", t.Group)
	cmdV, err := t.buildCommand(t.Array, cmdS)
	if err != nil {
		return err
	}
	if len(cmdV) < 2 {
		return fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}

	interval := 5 * time.Second
	defaultTimeout := 2 * time.Minute

	// The context should have the StartTimeout, but risk entering a
	// infinite loop.
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultTimeout)
	}

	for {
		retryable := false
		setRetryable := func(line string) {
			if strings.Contains(line, "currently being promoted") {
				retryable = true
			}
			if strings.Contains(line, "could be retried later") {
				retryable = true
			}
		}
		cmd := command.New(
			command.WithContext(ctx),
			command.WithName(cmdV[0]),
			command.WithArgs(cmdV[1:]),
			command.WithLogger(t.Log()),
			command.WithCommandLogLevel(zerolog.InfoLevel),
			command.WithStderrLogLevel(zerolog.ErrorLevel),
			command.WithStdoutLogLevel(zerolog.InfoLevel),
			command.WithOnStdoutLine(setRetryable),
			command.WithOnStderrLine(setRetryable),
		)
		err := cmd.Run()
		if err != nil {
			if retryable {
				t.Log().Infof("currently promoting, retry later")
				if deadline.Before(time.Now().Add(interval)) {
					return fmt.Errorf("timeout waiting for promote to finish")
				}
				time.Sleep(interval)
				continue
			} else {
				return err
			}
		}
		return nil
	}
	return nil
}

func (t *T) groupNames() (map[string]string, error) {
	m := make(map[string]string)
	obj, err := object.NewConfigurer(t.Path)
	if err != nil {
		return nil, err
	}
	nodes, err := obj.Config().NodeReferrer.Nodes()
	if err != nil {
		return nil, err
	}
	rid := t.RID()
	for _, node := range nodes {
		arrayName := obj.Config().GetStringAs(key.New(rid, "array"), node)
		groupName := obj.Config().GetStringAs(key.New(rid, "group"), node)
		m[arrayName] = groupName
	}
	return m, nil
}

func (t *T) runStopArrayRCG(ctx context.Context, target string) error {
	m, err := t.groupNames()
	if err != nil {
		return err
	}
	group, ok := m[target]
	if !ok {
		return fmt.Errorf("no can not determine group name on array %s, verify the resource config has group@<node> and array@<node> keywords set for all nodes.", target)
	}
	return t.runStopThisRCG(ctx, target, group)
}

func (t *T) runStopRCG(ctx context.Context) error {
	return t.runStopThisRCG(ctx, t.Array, t.Group)
}

func (t *T) syncArrayRCG(ctx context.Context, target string) error {
	m, err := t.groupNames()
	if err != nil {
		return err
	}
	group, ok := m[target]
	if !ok {
		return fmt.Errorf("no can not determine group name on array %s, verify the resource config has group@<node> and array@<node> keywords set for all nodes.", target)
	}
	return t.syncThisRCG(ctx, target, group)
}

func (t *T) syncThisRCG(ctx context.Context, arrayName, group string) error {
	cmdS := fmt.Sprintf("syncrcopy -w %s", group)
	cmdV, err := t.buildCommand(arrayName, cmdS)
	if err != nil {
		return err
	}
	if len(cmdV) < 2 {
		return fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cmdV[0]),
		command.WithArgs(cmdV[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

func (t *T) runStopThisRCG(ctx context.Context, arrayName, group string) error {
	cmdS := fmt.Sprintf("stoprcopygroup -f %s", group)
	cmdV, err := t.buildCommand(arrayName, cmdS)
	if err != nil {
		return err
	}
	if len(cmdV) < 2 {
		return fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cmdV[0]),
		command.WithArgs(cmdV[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

func (t *T) stopRCG(ctx context.Context) error {
	if t.groupStatus.Status == groupStatusStopped {
		t.Log().Infof("rcopy group %s is already stopped, skip stoprcopygroup", t.Group)
		return nil
	}

	if t.groupStatus.Role == groupRolePrimary {
		if err := t.runStopRCG(ctx); err != nil {
			return err
		}
	} else {
		if err := t.runStopArrayRCG(ctx, t.groupStatus.Target); err != nil {
			return err
		}
	}

	return nil
}

func (t *T) startGroup(ctx context.Context) error {
	if t.groupStatus.Status == groupStatusStarted {
		t.Log().Infof("rcopy group %s is already started, skip startrcopygroup", t.Group)
		return nil
	}

	if err := t.runStartGroup(ctx); err != nil {
		return err
	}

	return nil
}

func (t *T) syncRCG(ctx context.Context) error {
	if t.groupStatus.Role != groupRolePrimary {
		t.Log().Infof("rcopy group %s role is not Primary, skip sync", t.Group)
		return nil
	}

	if t.groupStatus.Mode == groupModePeriodic {
		t.Log().Infof("skip syncrcopy as group %s is in periodic mode", t.Group)
		return nil
	}

	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	unlock, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer unlock()

	if err := t.syncThisRCG(ctx, t.Array, t.Group); err != nil {
		return err
	}

	if err := t.waitRCGStatusSync(ctx); err != nil {
		return err
	}

	return nil
}

func (t *T) setRCGReverse(ctx context.Context) error {
	if t.groupStatus.Role == groupRolePrimary {
		t.Log().Infof("rcopy group %s role is already Primary, skip setrcopygroup reverse", t.Group)
		return nil
	}

	if err := t.reverse(ctx); err != nil {
		return err
	}

	return nil
}

// SubDevices returns the list of device files managed by this resource.
func (t *T) SubDevices(ctx context.Context) device.L {
	wwns, err := t.getGroupWWN(ctx)
	if err != nil {
		return device.L{}
	}
	var devs device.L
	for _, wwn := range wwns {
		devPath := "/dev/disk/by-id/scsi-3" + strings.ToLower(wwn)
		dest, err := filepath.EvalSymlinks(devPath)
		if err != nil {
			t.Log().Debugf("SubDevices: ReadLink(%s) ignored error: %s", devPath, err)
			continue
		}
		dev := device.New(dest, device.WithLogger(t.Log()))
		devs = append(devs, dev)
	}
	return devs
}

// promoteRW promotes the devices to read-write.
func (t *T) promoteRW(ctx context.Context) error {
	devs := t.SubDevices(ctx)
	t.Log().Tracef("devices to promote rw: %s", devs)
	for _, dev := range devs {
		if err := dev.PromoteRW(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (t *groupStatus) SyncStatus() (s string) {
	for _, vv := range t.Volumes {
		s = vv.SyncStatus
		if vv.SyncStatus != vvSyncStatusSynced {
			return
		}
	}
	return
}

// Oldest returns the oldest LastSyncTime of all VV.
// For Sync mode all VV LastSyncTime are zero, so Oldest will return zero.
func (t *groupStatus) Oldest() time.Time {
	var oldest time.Time
	for i, vv := range t.Volumes {
		if vv.LastSyncTime.IsZero() {
			// In Sync mode, VV LastSyncTime is reported as "NA"
			// parsed as a zero time.
			continue
		}
		if i == 0 || vv.LastSyncTime.Before(oldest) {
			oldest = vv.LastSyncTime
		}
	}
	return oldest
}

func (t *groupStatus) Period() (time.Duration, error) {
	var period time.Duration
	for _, opt := range t.Options {
		if strings.HasPrefix(opt, "Period") {
			fields := strings.Fields(opt)
			if len(fields) < 2 {
				return period, fmt.Errorf("unexpected number of fields in RCG option: %s", opt)
			}
			return time.ParseDuration(fields[1])
		}
	}
	return period, nil
}

func (t *groupStatus) String() string {
	var l []string
	l = append(l, "role:"+t.Role)
	l = append(l, "state:"+t.Status)
	l = append(l, "mode:"+t.Mode)
	if t.Mode == groupModePeriodic {
		age := time.Now().Sub(t.Oldest())
		l = append(l, fmt.Sprintf("age:%s", duration.FmtShortDuration(age)))
	}
	return strings.Join(l, " ")
}
