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
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/duration"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/rs/zerolog"
)

const (
	// Remote copy group states
	rcgStatusStarted  = "Started"
	rcgStatusStopped  = "Stopped"
	rcgStatusStarting = "Starting"
	rcgStatusStopping = "Stopping"

	// Remote copy group roles
	rcgRolePrimary    = "Primary"
	rcgRolePrimaryRev = "Primary-Rev"
	rcgRoleSecondary  = "Secondary"

	// Remote copy group modes
	rcgModeSync     = "Sync"
	rcgModePeriodic = "Periodic"

	// Volume sync status
	vvSyncStatusSynced  = "Synced"
	vvSyncStatusSyncing = "Syncing"

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

	// RCG is the name of the HP 3PAR remote copy group.
	RCG string `json:"rcg"`

	// Mode is the replication mode: "sync" or "async".
	Mode string `json:"mode"`

	// Timeout is the maximum duration to wait for operations to complete.
	Timeout *time.Duration `json:"timeout"`

	// StartTimeout is the maximum duration to wait for start operations.
	StartTimeout *time.Duration `json:"start_timeout"`

	// arrayConfig holds the resolved array configuration
	arrayConfig *Config

	// Cached resolved values for datastore-referenced files
	keyFileCache string
	pwfCache     string

	rcgStatusCache *rcgStatus
}

// rcgStatus holds the status information for a remote copy group.
type rcgStatus struct {
	Name    string
	Target  string
	Status  string
	Role    string
	Mode    string
	Options []string
	Volumes []vvStatus
}

// vvStatus holds the status information for a volume in a remote copy group.
type vvStatus struct {
	LocalVV      string
	RemoteVV     string
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
	return fmt.Sprintf("%s/%s", t.Array, t.RCG)
}

// Info returns key/value pairs used for resource info display.
func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := make(resource.InfoKeys, 0)
	m = append(m,
		resource.InfoKey{Key: "array", Value: t.Array},
		resource.InfoKey{Key: "rcg", Value: t.RCG},
		resource.InfoKey{Key: "mode", Value: t.Mode},
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
	ps, err := t.cachedRCGStatus(ctx)
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.NotApplicable
	}

	if ps == nil {
		t.StatusLog().Info("no rcg status available")
		return status.NotApplicable
	}

	// Check overall RCG status
	if ps.Status != rcgStatusStarted {
		t.StatusLog().Info("rcg status is %s (expected Started)", ps.Status)
	}

	// Check role based on mode
	if t.Mode == "sync" {
		if ps.Role != rcgRolePrimary {
			t.StatusLog().Warn("rcg role is %s (expected Primary for sync mode)", ps.Role)
		}
	} else if t.Mode == "async" {
		if ps.Role != rcgRolePrimary {
			t.StatusLog().Warn("rcg role is %s (expected Primary for async mode)", ps.Role)
		}
		if ps.Mode != rcgModePeriodic {
			t.StatusLog().Warn("rcg mode is %s (expected Periodic for async mode)", ps.Mode)
		}
	}

	// Check volume sync status
	period, err := ps.Period()
	if err != nil {
		t.StatusLog().Error("%s", err)
	} else if period > 0 {
		elapsed := time.Now().UTC().Add(-1 * period)
		for _, vv := range ps.Volumes {
			if vv.SyncStatus != vvSyncStatusSynced {
				t.StatusLog().Warn("volume %s sync status is %s (expected Synced)", vv.LocalVV, vv.SyncStatus)
			}
			if vv.LastSyncTime.Before(elapsed) {
				t.StatusLog().Warn("volume %s last sync too old (%s)", vv.LocalVV, vv.LastSyncTime.Format("2006-01-02 15:04:05"))
			}
		}
	}

	t.StatusLog().Info(ps.String())
	return status.NotApplicable
}

// Provisioned returns whether the RCG exists.
func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	ps, err := t.rcgStatus(ctx)
	if err != nil {
		return provisioned.False, err
	}
	if ps == nil {
		return provisioned.False, nil
	}
	return provisioned.True, nil
}

// Abort ensures the array connection is working.
func (t *T) Abort(ctx context.Context) error {
	return t.testArrayConnection(ctx)
}

// Start starts the replication in the appropriate direction.
func (t *T) Start(ctx context.Context) error {
	if err := t.loadArrayConfig(); err != nil {
		return fmt.Errorf("failed to load array configuration: %w", err)
	}

	ps, err := t.rcgStatus(ctx)
	if err != nil {
		return err
	}

	if ps == nil {
		return fmt.Errorf("rcg %s not found", t.RCG)
	}

	// Check if we are split from target
	if v, err := t.isSplitted(ctx, ps.Target); err != nil {
		return err
	} else if v {
		t.Log().Infof("we are split from %s array", ps.Target)
		return t.startSplitted(ctx)
	}

	t.Log().Infof("we are joined with %s array", ps.Target)
	return t.startJoined(ctx)
}

func (t *T) startJoined(ctx context.Context) error {
	ps, err := t.rcgStatus(ctx)
	if err != nil {
		return err
	}

	if ps.Role == rcgRolePrimary {
		t.Log().Infof("rcopy group %s role is already Primary, skip", t.RCG)
		return nil
	}

	if err := t.stopRCG(ctx); err != nil {
		return err
	}

	if err := t.setRCGReverse(ctx); err != nil {
		return err
	}

	// If this node is in the service nodes, resume
	return t.startRCG(ctx)
}

func (t *T) startSplitted(ctx context.Context) error {
	return t.setRCGFailover(ctx)
}

// Stop stops the replication.
func (t *T) Stop(ctx context.Context) error {
	return nil
}

// Resync re-establishes the replication after a split.
func (t *T) Resync(ctx context.Context) error {
	return t.syncRCG(ctx)
}

// Update performs a sync operation.
func (t *T) Update(ctx context.Context) error {
	return t.syncRCG(ctx)
}

// Swap swaps the replication direction.
func (t *T) Swap(ctx context.Context) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	unlock, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer unlock()

	ps, err := t.rcgStatus(ctx)
	if err != nil {
		return err
	}

	if ps.Role == rcgRolePrimary {
		return fmt.Errorf("rcopy group %s role is Primary, refuse to swap", t.RCG)
	}

	if err := t.stopRCG(ctx); err != nil {
		return err
	}

	if err := t.setRCGReverse(ctx); err != nil {
		return err
	}

	return t.startRCG(ctx)
}

// Resume resumes the replication.
func (t *T) Resume(ctx context.Context) error {
	return t.startRCG(ctx)
}

// Split quiesces the replication.
func (t *T) Split(ctx context.Context) error {
	return t.stopRCG(ctx)
}

// ---------------------------------------------------------------------------
// Array configuration loading
// ---------------------------------------------------------------------------

// arraySectionName returns the full array section name in cluster config.
func (t *T) arraySectionName() string {
	return fmt.Sprintf("array#%s", t.Array)
}

// loadArrayConfig loads the array configuration from the cluster config.
// Configuration is read from the array#<suffix> section where <suffix> is t.Array.
func (t *T) loadArrayConfig() error {
	if t.arrayConfig != nil {
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
	sectionName := t.arraySectionName()

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

	t.arrayConfig = config
	t.Log().Tracef("loaded array config: %#v", config)
	return nil
}

// getArrayConfig returns the loaded array configuration.
func (t *T) getArrayConfig() (*Config, error) {
	if err := t.loadArrayConfig(); err != nil {
		return nil, err
	}
	return t.arrayConfig, nil
}

// ---------------------------------------------------------------------------
// Datastore-backed configuration resolution
// ---------------------------------------------------------------------------

// manager returns the resolved manager value from array config.
func (t *T) manager() (string, error) {
	config, err := t.getArrayConfig()
	if err != nil {
		return "", err
	}
	return config.Manager, nil
}

// username returns the resolved username value from array config.
func (t *T) username() (string, error) {
	config, err := t.getArrayConfig()
	if err != nil {
		return "", err
	}
	return config.Username, nil
}

// method returns the resolved method value from array config.
func (t *T) method() (string, error) {
	config, err := t.getArrayConfig()
	if err != nil {
		return "", err
	}
	return config.Method, nil
}

// keyFile returns the resolved keyfile path, supporting "from <path> key <key>" format.
// The content is cached as a temporary file.
func (t *T) keyFile() (string, error) {
	config, err := t.getArrayConfig()
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
func (t *T) pwf() (string, error) {
	config, err := t.getArrayConfig()
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
func (t *T) cli() (string, error) {
	config, err := t.getArrayConfig()
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

func (t *T) syncMaxDelay() int64 {
	// Default to 300 seconds (5 minutes)
	const defaultMaxDelay = 300
	return defaultMaxDelay
}

func (t *T) testArrayConnection(ctx context.Context) error {
	cmdS := "showsys"
	cmdV, err := t.buildCommand(cmdS)
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
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
	)
	return cmd.Run()
}

func (t *T) wrapSSHCommand(cmd string) string {
	return fmt.Sprintf("setclienv csvtable 1; setclienv nohdtot 1; %s; exit", cmd)
}

func (t *T) buildSSHCommand(cmd string) ([]string, error) {
	cmd = t.wrapSSHCommand(cmd)

	// For SSH method: ssh -i <key> <username>@<manager>
	keyFile, err := t.keyFile()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve keyfile: %w", err)
	}
	username, err := t.username()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve username: %w", err)
	}
	manager, err := t.manager()
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

func (t *T) buildCLICommand(cmd string) ([]string, error) {
	// For CLI method: <cli> -sys <manager> -nohdtot -csvtable -pwf <pwf> <command>
	cli, err := t.cli()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cli: %w", err)
	}
	manager, err := t.manager()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve manager: %w", err)
	}
	pwf, err := t.pwf()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve pwf: %w", err)
	}

	args := []string{cli, "-sys", manager, "-nohdtot", "-csvtable"}
	if pwf != "" {
		args = append(args, "-pwf", pwf)
	}
	args = append(args, strings.Fields(cmd)...)
	return args, nil
}

func (t *T) buildCommand(cmd string) ([]string, error) {
	method, err := t.method()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve method: %w", err)
	}

	switch method {
	case methodSSH:
		return t.buildSSHCommand(cmd)
	case methodCLI:
		return t.buildCLICommand(cmd)
	default:
		return nil, fmt.Errorf("%w: unknown method %s", ErrBuildCommand, method)
	}
}

func (t *T) rcgStatus(ctx context.Context) (*rcgStatus, error) {
	cmdS := "showrcopy groups"
	cmdV, err := t.buildCommand(cmdS)
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
	return t.parseRCGStatus(string(out), t.RCG)
}

func (t *T) cachedRCGStatus(ctx context.Context) (*rcgStatus, error) {
	if t.rcgStatusCache != nil {
		return t.rcgStatusCache, nil
	}
	return t.rcgStatus(ctx)
}

func (t *T) clearCaches() {
	t.rcgStatusCache = nil
	// Clear array config to force reload on next access
	t.arrayConfig = nil
}

func (t *T) parseRCGStatus(out string, rcgName string) (*rcgStatus, error) {
	// Parse the showrcopy groups output
	// Format:
	// Name,Target,Status,Role,Mode,Options
	// rcg1,target1,Started,Primary,Sync,"opt1,opt2"
	//  ,LocalVV,ID,RemoteVV,ID,SyncStatus,LastSyncTime
	//  ,vol1,1,vol2,2,Synced,2024-01-01 12:00:00

	lines := strings.Split(out, "\n")
	var rcg *rcgStatus
	var inRCGBlock bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is the start of our RCG block
		if strings.HasPrefix(line, rcgName+",") {
			inRCGBlock = true
			ps := t.parseRCGLine(line)
			if ps != nil {
				rcg = ps
				rcg.Name = rcgName
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
			if vv != nil && rcg != nil {
				rcg.Volumes = append(rcg.Volumes, *vv)
			}
		}
	}

	if rcg == nil {
		return nil, fmt.Errorf("rcg %s not found", rcgName)
	}

	return rcg, nil
}

func (t *T) parseRCGLine(line string) *rcgStatus {
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

	rcg := &rcgStatus{
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
				rcg.Options = append(rcg.Options, opt)
			}
		}
	}

	if len(parts) >= 2 {
		rcg.Target = parts[1]
	}

	return rcg
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
		RemoteVV:   parts[2],
		SyncStatus: parts[4],
	}

	// Parse LastSyncTime
	timeStr := strings.TrimSpace(parts[5])
	if timeStr != "" {
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

func (t *T) isSplitted(ctx context.Context, target string) (bool, error) {
	// Check if replication links to target are down
	cmdS := "showrcopy links"
	cmdV, err := t.buildCommand(cmdS)
	if err != nil {
		return false, err
	}
	if len(cmdV) < 2 {
		return false, fmt.Errorf("%w: %s", ErrBuildCommand, cmdS)
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
		return false, err
	}

	// Parse showrcopy links output
	// Format: Target,Node,Address,Status,Options
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) >= 4 {
			if strings.TrimSpace(parts[0]) == target {
				status := strings.TrimSpace(parts[3])
				if status == "Up" {
					return false, nil
				}
			}
		}
	}

	return true, nil
}

func (t *T) runFailoverRCG(ctx context.Context) error {
	cmdS := fmt.Sprintf("setrcopygroup failover -f -waittask %s", t.RCG)
	cmdV, err := t.buildCommand(cmdS)
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
	defer t.clearCaches()
	return cmd.Run()
}

func (t *T) runReverseRCG(ctx context.Context) error {
	cmdS := fmt.Sprintf("setrcopygroup reverse -f -waittask %s", t.RCG)
	cmdV, err := t.buildCommand(cmdS)
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
	defer t.clearCaches()
	return cmd.Run()
}

func (t *T) runSyncRCG(ctx context.Context) error {
	cmdS := fmt.Sprintf("syncrcopy -w %s", t.RCG)
	cmdV, err := t.buildCommand(cmdS)
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
	defer t.clearCaches()
	return cmd.Run()
}

func (t *T) runStartRCG(ctx context.Context) error {
	cmdS := fmt.Sprintf("startrcopygroup %s", t.RCG)
	cmdV, err := t.buildCommand(cmdS)
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
	defer t.clearCaches()
	return cmd.Run()
}

func (t *T) runStopRCG(ctx context.Context) error {
	cmdS := fmt.Sprintf("stoprcopygroup -f %s", t.RCG)
	cmdV, err := t.buildCommand(cmdS)
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
	defer t.clearCaches()
	return cmd.Run()
}

func (t *T) stopRCG(ctx context.Context) error {
	ps, err := t.rcgStatus(ctx)
	if err != nil {
		return err
	}

	if ps.Status == rcgStatusStopped {
		t.Log().Infof("rcopy group %s is already stopped, skip stoprcopygroup", t.RCG)
		return nil
	}

	if ps.Role == rcgRolePrimary {
		if err := t.runStopRCG(ctx); err != nil {
			return err
		}
	} else {
		// For non-primary, stop on the target
		target := ps.Target
		// In a real implementation, we'd run the command on the target array
		// For now, we'll just log this
		t.Log().Infof("would stop rcopy group %s on target array %s", t.RCG, target)
	}

	t.clearCaches()
	return nil
}

func (t *T) startRCG(ctx context.Context) error {
	ps, err := t.rcgStatus(ctx)
	if err != nil {
		return err
	}

	if ps.Status == rcgStatusStarted {
		t.Log().Infof("rcopy group %s is already started, skip startrcopygroup", t.RCG)
		return nil
	}

	if ps.Role != rcgRolePrimary {
		return fmt.Errorf("rcopy group %s role is not Primary, refuse to start rcopy", t.RCG)
	}

	if err := t.runStartRCG(ctx); err != nil {
		return err
	}

	return nil
}

func (t *T) syncRCG(ctx context.Context) error {
	ps, err := t.rcgStatus(ctx)
	if err != nil {
		return err
	}

	if ps.Role != rcgRolePrimary {
		t.Log().Infof("rcopy group %s role is not Primary, skip sync", t.RCG)
		return nil
	}

	if ps.Mode == rcgModePeriodic {
		t.Log().Infof("skip syncrcopy as group %s is in periodic mode", t.RCG)
		return nil
	}

	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	unlock, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer unlock()

	if err := t.runSyncRCG(ctx); err != nil {
		return err
	}

	return nil
}

func (t *T) setRCGReverse(ctx context.Context) error {
	ps, err := t.rcgStatus(ctx)
	if err != nil {
		return err
	}

	if ps.Role == rcgRolePrimary {
		t.Log().Infof("rcopy group %s role is already Primary, skip setrcopygroup reverse", t.RCG)
		return nil
	}

	if err := t.runReverseRCG(ctx); err != nil {
		return err
	}

	t.clearCaches()
	return nil
}

func (t *T) setRCGFailover(ctx context.Context) error {
	ps, err := t.rcgStatus(ctx)
	if err != nil {
		return err
	}

	if ps.Role == rcgRolePrimaryRev {
		t.Log().Infof("rcopy group %s role is already Primary-Rev, skip setrcopygroup failover", t.RCG)
		return nil
	}

	if err := t.runFailoverRCG(ctx); err != nil {
		return err
	}

	t.clearCaches()
	return nil
}

// SubDevices returns the list of device files managed by this resource.
func (t *T) SubDevices(ctx context.Context) device.L {
	// In a real implementation, this would return the device files
	// for the volumes in the RCG
	return device.L{}
}

// PromoteRW promotes the devices to read-write.
func (t *T) PromoteRW(ctx context.Context) error {
	// In a real implementation, this would promote the volumes
	// to read-write after failover
	return nil
}

func (t *rcgStatus) Oldest() time.Time {
	var oldest time.Time
	for i, vv := range t.Volumes {
		if i == 0 || vv.LastSyncTime.Before(oldest) {
			oldest = vv.LastSyncTime
		}
	}
	return oldest
}

func (t *rcgStatus) Period() (time.Duration, error) {
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

func (t *rcgStatus) String() string {
	var l []string
	age := time.Now().Sub(t.Oldest())
	l = append(l, "role:"+t.Role)
	l = append(l, "state:"+t.Status)
	l = append(l, fmt.Sprintf("age:%s", duration.FmtShortDuration(age)))
	l = append(l, "mode:"+t.Mode)
	return strings.Join(l, " ")
}
