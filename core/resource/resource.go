package resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-collections/collections/set"
	"github.com/google/uuid"
	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/resourcereqs"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/statusbus"
	"github.com/opensvc/om3/v3/core/trigger"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/runfiles"
	"github.com/opensvc/om3/v3/util/scsi"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	ObjectDriver interface {
		Log() *plog.Logger
		VarDir() string
		ResourceByID(string) Driver
		ResourcesByDrivergroups([]driver.Group) Drivers
	}

	Setenver interface {
		Setenv()
	}

	StatusLogger interface {
		Info(string, ...any)
		Warn(string, ...any)
		Error(string, ...any)
		Len() int
		Reset()
		Entries() []StatusLogEntry
		Merge(StatusLogger)
	}

	// Driver exposes what can be done with a resource
	Driver interface {
		Provisioned(context.Context) (provisioned.T, error)
		Provision(context.Context) error
		Unprovision(context.Context) error

		// common
		ApplyPGChain(context.Context) error
		GetConfigurationError() error
		GetObject() any
		GetPG() *pg.Config
		GetPGID() string
		ID() *resourceid.T
		IsActionDisabled() bool
		IsConfigured() bool
		IsDisabled() bool
		IsEncap() bool
		IsMonitored() bool
		IsOptional() bool
		IsProvisionDisabled() bool
		IsUnprovisionDisabled() bool
		IsShared() bool
		IsStandby() bool
		IsStopped() bool
		IsStatusDisabled() bool
		SetConfigured(error)

		// Label returns a formatted short description of the Resource
		Label(context.Context) string

		Log() *plog.Logger
		Manifest() *manifest.T
		MatchRID(string) bool
		MatchSubset(string) bool
		MatchTag(string) bool
		Requires(string) *resourcereqs.T
		RID() string
		RSubset() string
		SetObject(any)
		SetPG(*pg.Config)
		SetRID(string) error
		Status(context.Context) status.T
		StatusLog() StatusLogger
		TagSet() TagSet
		Trigger(context.Context, trigger.Blocking, trigger.Hook, trigger.Action) error
		VarDir() string
	}

	Restart struct {
		// Count is how many times imon should try to restart before giving up.
		Count int

		// Delay is the duration between 2 restarts.
		Delay *time.Duration
	}

	// T is the resource type, embedded in each drivers type
	T struct {
		Driver
		ResourceID              *resourceid.T
		Subset                  string
		Disable                 bool
		Monitor                 bool
		Optional                bool
		Standby                 bool
		Shared                  bool
		Encap                   bool
		Tags                    *set.Set
		BlockingPreStart        string
		BlockingPreStop         string
		BlockingPreRun          string
		BlockingPreProvision    string
		BlockingPreUnprovision  string
		PreStart                string
		PreStop                 string
		PreRun                  string
		PreProvision            string
		PreUnprovision          string
		BlockingPostStart       string
		BlockingPostStop        string
		BlockingPostRun         string
		BlockingPostProvision   string
		BlockingPostUnprovision string
		PostStart               string
		PostStop                string
		PostRun                 string
		PostProvision           string
		PostUnprovision         string
		StartRequires           string
		StopRequires            string
		ProvisionRequires       string
		UnprovisionRequires     string
		SyncRequires            string
		RunRequires             string
		EnableProvision         bool
		EnableUnprovision       bool

		configurationError error
		statusLog          StatusLog
		log                plog.Logger
		object             any
		objectDriver       ObjectDriver
		pg                 *pg.Config
	}

	// devReservabler is an interface implemented by resource drivers that want the core resource
	// to handle SCSI persistent reservation on a list of devices.
	devReservabler interface {
		// ReservableDevices must be implement by every driver that wants SCSI PR.
		ReservableDevices(context.Context) device.L

		// IsSCSIPersistentReservationPreemptAbortDisabled is exposing the resource no_preempt_abort keyword value.
		IsSCSIPersistentReservationPreemptAbortDisabled() bool

		// IsSCSIPersistentReservationEnabled is exposing the scsireserv resource keyword value.
		IsSCSIPersistentReservationEnabled() bool

		// PersistentReservationKey is exposing the prkey resource keyword value.
		PersistentReservationKey() string
	}

	devImporter interface {
		ImportDevices(context.Context) error
	}

	SCSIPersistentReservation struct {
		Key            string
		NoPreemptAbort bool
		Enabled        bool
	}

	// ProvisionStatus define if and when the resource became provisioned.
	ProvisionStatus struct {
		Mtime time.Time     `json:"mtime,omitempty"`
		State provisioned.T `json:"state"`
	}

	// TagSet is the list of unique tag names found in the resource definition.
	TagSet []string

	// Status is the structure representing the resource status,
	// which is embedded in the instance status.
	Status struct {
		ResourceID    *resourceid.T    `json:"-"`
		Label         string           `json:"label"`
		Log           []StatusLogEntry `json:"log,omitempty"`
		Status        status.T         `json:"status"`
		Type          string           `json:"type"`
		IsProvisioned ProvisionStatus  `json:"provisioned,omitempty"`
		IsMonitored   bool             `json:"monitor,omitempty"`
		IsDisabled    bool             `json:"disable,omitempty"`
		IsOptional    bool             `json:"optional,omitempty"`
		IsEncap       bool             `json:"encap,omitempty"`
		IsStandby     bool             `json:"standby,omitempty"`
		IsStopped     bool             `json:"stopped,omitempty"`

		// Subset is the name of the subset this resource is assigned to.
		Subset string `json:"subset,omitempty"`

		// Info is a list of key-value pairs providing interesting information to
		// collect site-wide about this resource.
		Info map[string]any `json:"info,omitempty"`

		// Tags is a set of words attached to the resource.
		Tags TagSet `json:"tags,omitempty"`

		Files Files `json:"files,omitempty"`
	}

	Files []File
	File  struct {
		Checksum string    `json:"csum"`
		Mtime    time.Time `json:"mtime"`
		Name     string    `json:"name"`
		Ingest   bool      `json:"ingest"`
	}

	// RunningInfoList is the list of the in-progress run info (for sync and task).
	RunningInfoList []RunningInfo

	// RunningInfo describes a run in progress (for sync and task).
	RunningInfo struct {
		PID       int       `json:"pid"`
		RID       string    `json:"rid"`
		SessionID uuid.UUID `json:"session_id"`
		At        time.Time `json:"at"`
	}

	Hook int

	StatusInfoSchedAction struct {
		Last time.Time `json:"last"`
	}

	// ScheduleOptions contains the information needed by the object to create a
	// schedule.Entry to append to the object's schedule.Table.
	ScheduleOptions struct {
		Action              string
		MaxParallel         int
		Option              string
		Base                string
		RequireCollector    bool
		RequireProvisioned  bool
		RequireConfirmation bool
		RunDir              string
		Require             string
	}
)

const (
	Pre Hook = iota
	Post
)

var (
	ErrActionNotSupported      = errors.New("the resource action is not supported on resource")
	ErrActionPostponedToLinker = errors.New("the resource action is postponed to its linker")
	ErrDisabled                = errors.New("the resource is disabled")
	ErrActionReqNotMet         = errors.New("the resource action requirements are not met")
	ErrBarrier                 = errors.New("barrier hit")
)

func (t *Restart) GetRestart() Restart {
	return *t
}

func (t *Restart) SetRestart(n *Restart) {
	t = n
}

// IsMonitoredFlag returns a one character representation of the IsMonitored state.
func (t *Status) IsMonitoredFlag() string {
	if t.IsMonitored {
		return "M"
	}
	return "."
}

// IsDisabledFlag returns a one character representation of the IsDisabled state.
func (t *Status) IsDisabledFlag() string {
	if t.IsDisabled {
		return "D"
	}
	return "."
}

// RestartFlag returns a one character representation of the Restart state.
func (t *Status) RestartFlag(restart, retries int) string {
	switch {
	case t.IsStopped:
		return "X"
	case restart <= 0:
		return "."
	case retries <= 0:
		return "0"
	case retries < 10:
		return fmt.Sprintf("%d", retries)
	default:
		return "+"
	}
}

// IsOptionalFlag returns a one character representation of the IsOptional state.
func (t *Status) IsOptionalFlag() string {
	if t.IsOptional {
		return "O"
	}
	return "."
}

// IsEncapFlag returns a one character representation of the IsEncap state.
func (t *Status) IsEncapFlag() string {
	if t.IsEncap {
		return "E"
	}
	return "."
}

// IsStandbyFlag returns a one character representation of the IsStandby state.
func (t *Status) IsStandbyFlag() string {
	if t.IsStandby {
		return "S"
	}
	return "."
}

// IsProvisionedFlag returns a one character representation of the IsProvisioned state.
func (t *Status) IsProvisionedFlag() string {
	return t.IsProvisioned.State.FlagString()
}

func NewResourceFunc(drvID driver.ID) func() Driver {
	drv, ok := driver.Get(drvID)
	if !ok {
		return nil
	}
	if a, ok := drv.Allocator.(func() Driver); ok {
		return a
	}
	return nil
}

// IsOptional returns true if the resource definition contains optional=true.
// An optional resource does not break an object action on error.
//
// Resource having actions disabled are always considered optional, because
// there is nothing we can do to change the state, which would cause
// orchestration loops.
func (t *T) IsOptional() bool {
	if t.IsActionDisabled() {
		return true
	}
	return t.Optional
}

// IsEncap returns true if the resource definition contains encap=true.
func (t *T) IsEncap() bool {
	return t.Encap || t.MatchTag("encap")
}

// IsDisabled returns true if the resource definition contains disable=true.
func (t *T) IsDisabled() bool {
	return t.Disable
}

// IsProvisionDisabled returns true if the resource definition contains provision=false.
func (t *T) IsProvisionDisabled() bool {
	return !t.EnableProvision
}

// IsUnprovisionDisabled returns true if the resource definition contains unprovision=false.
func (t *T) IsUnprovisionDisabled() bool {
	return !t.EnableUnprovision
}

// IsStandby returns true if the resource definition contains standby=true.
func (t *T) IsStandby() bool {
	return t.Standby
}

// IsStopped returns true if the stopped flag file exists.
func (t *T) IsStopped() bool {
	v, _ := IsStopped(t)
	return v
}

// IsShared returns true if the resource definition contains shared=true.
func (t *T) IsShared() bool {
	return t.Shared
}

// IsMonitored returns true if the resource definition contains monitor=true.
func (t *T) IsMonitored() bool {
	return t.Monitor
}

// IsStatusDisabled returns true if the resource definition contains tag=nostatus ...
// In this case, the resource status is always n/a
func (t *T) IsStatusDisabled() bool {
	return t.MatchTag("nostatus")
}

// IsActionDisabled returns true if the resource definition contains tag=noaction ...
// In this case, the resource actions like stop and start are skipped.
func (t *T) IsActionDisabled() bool {
	return t.MatchTag("noaction")
}

func (t *T) IsConfigured() bool {
	return t.configurationError == nil
}

func (t *T) GetConfigurationError() error {
	return t.configurationError
}

func (t *T) SetConfigured(err error) {
	t.configurationError = err
}

// RSubset returns the resource subset name
func (t *T) RSubset() string {
	return t.Subset
}

// StatusLog returns a reference to the resource log
func (t *T) StatusLog() StatusLogger {
	return &t.statusLog
}

// RID returns the string representation of the resource id
func (t *T) RID() string {
	return t.ResourceID.String()
}

// ID returns the resource id struct
func (t *T) ID() *resourceid.T {
	return t.ResourceID
}

// SetRID sets the resource identifier
func (t *T) SetRID(v string) error {
	rid, err := resourceid.Parse(v)
	if err != nil {
		return err
	}
	t.ResourceID = rid
	return nil
}

// SetPG sets the process group config parsed from the config
func (t *T) SetPG(v *pg.Config) {
	t.pg = v
}

// GetPG returns the private pg resource field
func (t *T) GetPG() *pg.Config {
	return t.pg
}

// GetPGID returns the pg id configured via SetPG, or "" if unset
func (t *T) GetPGID() string {
	if t.pg == nil {
		return ""
	}
	return t.pg.ID
}

// ApplyPGChain fetches the pg manager from the action context and
// apply the pg configuration to all unconfigured pg on the pg id
// hierarchy (resource=>subset=>object).
//
// The pg manager remembers which pg have been configured to avoid
// doing the config twice.
func (t *T) ApplyPGChain(ctx context.Context) error {
	mgr := pg.FromContext(ctx)
	if mgr == nil {
		// probably testing
		return nil
	}
	var errs error
	for _, run := range mgr.Apply(t.GetPGID()) {
		if !run.Changed {
			continue
		}
		if configStr := run.Config.String(); strings.Contains(configStr, "=") {
			t.Log().Infof("applied %s", configStr)
		} else {
			t.Log().Tracef("create %s", configStr)
		}
		if run.Err != nil {
			errs = errors.Join(errs, run.Err)
		}
	}
	return errs
}

// SetObject holds the useful interface of the parent object of the resource.
func (t *T) SetObject(o any) {
	if od, ok := o.(ObjectDriver); !ok {
		panic("SetObject accepts only ObjectDriver")
	} else {
		t.object = o
		t.log = *t.getLoggerFromObjectDriver(od)
	}
}

// GetObject returns the object interface set by SetObjectriver upon configure.
func (t *T) GetObject() any {
	return t.object
}

// GetObjectDriver returns the object driver interface of the object set by SetObject upon configure.
func (t *T) GetObjectDriver() ObjectDriver {
	return t.object.(ObjectDriver)
}

func (t *T) getLoggerFromObjectDriver(o ObjectDriver) *plog.Logger {
	oLog := o.Log()
	prefix := fmt.Sprintf("%s%s: ", oLog.Prefix(), t.ResourceID)
	l := plog.NewLogger(oLog.Logger()).WithPrefix(prefix).Attr("rid", t.ResourceID)
	if t.Subset != "" {
		l = l.Attr("subset", t.Subset)
	}
	return l
}

// Log returns the resource logger
func (t *T) Log() *plog.Logger {
	return &t.log
}

// MatchRID returns true if:
//
//   - the pattern is a just a drivergroup name and this name matches this resource's drivergroup
//     ex: fs#1 matches fs
//   - the pattern is a fully qualified resourceid, and its string representation equals the
//     pattern.
//     ex: fs#1 matches fs#1
func (t *T) MatchRID(s string) bool {
	return t.ResourceID.Match(s)

}

// MatchSubset returns true if the resource subset equals the pattern.
func (t *T) MatchSubset(s string) bool {
	return t.Subset == s
}

// MatchTag returns true if one of the resource tags equals the pattern.
func (t *T) MatchTag(s string) bool {
	if t.Tags == nil {
		return false
	}
	return t.Tags.Has(s)
}

func (t *T) TagSet() TagSet {
	s := make(TagSet, 0)
	t.Tags.Do(func(e any) { s = append(s, e.(string)) })
	return s
}

func formatResourceLabel(ctx context.Context, r Driver) string {
	name := r.Manifest().DriverID.Name
	if name == "" {
		return r.Label(ctx)
	} else {
		return strings.Join([]string{name, r.Label(ctx)}, " ")
	}
}

func (t *T) trigger(ctx context.Context, s string) error {
	cmdArgs, err := command.CmdArgsFromString(s)
	if err != nil {
		return err
	}
	if len(cmdArgs) == 0 {
		return nil
	}
	cmd := command.New(
		command.WithName(cmdArgs[0]),
		command.WithVarArgs(cmdArgs[1:]...),
		command.WithLogger(&t.log),
		command.WithEnv(append(os.Environ(), "OPENSVC_RID="+t.RID())),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel))
	return cmd.Run()
}

func (t *T) Trigger(ctx context.Context, blocking trigger.Blocking, hook trigger.Hook, action trigger.Action) error {
	var cmd string
	hookID := trigger.Format(blocking, hook, action)
	switch hookID {
	case "blocking_pre_start":
		cmd = t.BlockingPreStart
	case "pre_start":
		cmd = t.PreStart
	case "blocking_post_start":
		cmd = t.BlockingPostStart
	case "post_start":
		cmd = t.PostStart
	//
	case "blocking_pre_stop":
		cmd = t.BlockingPreStop
	case "pre_stop":
		cmd = t.PreStop
	case "blocking_post_stop":
		cmd = t.BlockingPostStop
	case "post_stop":
		cmd = t.PostStop
	//
	case "blocking_pre_run":
		cmd = t.BlockingPreRun
	case "pre_run":
		cmd = t.PreRun
	case "blocking_post_run":
		cmd = t.BlockingPostRun
	case "post_run":
		cmd = t.PostRun
	//
	case "blocking_pre_provision":
		cmd = t.BlockingPreProvision
	case "pre_provision":
		cmd = t.PreProvision
	case "blocking_post_provision":
		cmd = t.BlockingPostProvision
	case "post_provision":
		cmd = t.PostProvision
	//
	case "blocking_pre_unprovision":
		cmd = t.BlockingPreUnprovision
	case "pre_unprovision":
		cmd = t.PreUnprovision
	case "blocking_post_unprovision":
		cmd = t.BlockingPostUnprovision
	case "post_unprovision":
		cmd = t.PostUnprovision
	default:
		return nil
	}
	if cmd == "" {
		return nil
	}
	t.Log().Infof("trigger %s %s %s: %s", blocking, hook, action, cmd)
	return t.trigger(ctx, cmd)
}

func (t *T) Requires(action string) *resourcereqs.T {
	reqs := ""
	switch action {
	case "start":
		reqs = t.StartRequires
	case "stop":
		reqs = t.StopRequires
	case "provision":
		reqs = t.ProvisionRequires
	case "unprovision":
		reqs = t.UnprovisionRequires
	case "run":
		reqs = t.RunRequires
	case "sync":
		reqs = t.SyncRequires
	}
	return resourcereqs.New(reqs)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode()
	}
	return 0
}

func Setenv(r Driver) {
	if s, ok := r.(Setenver); ok {
		s.Setenv()
	}
}

func StatusCheckRequires(ctx context.Context, action string, r Driver) error {
	reqs := r.Requires(action)
	sb := statusbus.FromContext(ctx)
	for rid, reqStates := range reqs.Requirements() {
		state := sb.Get(rid)
		if state == status.Undef {
			return fmt.Errorf("invalid requirement: resource '%s' does not exist (syntax: <rid>(<state>[,<state])", rid)
		}
		if reqStates.Has(state) {
			continue // requirement met
		}
		return fmt.Errorf("%w: action %s on resource %s requires %s in states (%s), but is %s", ErrActionReqNotMet, action, r.RID(), rid, reqStates, state)
	}
	// all requirements met
	return nil
}

func checkRequires(ctx context.Context, r Driver) error {
	props := actioncontext.Props(ctx)
	reqs := r.Requires(props.Name)
	sb := statusbus.FromContext(ctx)
	for rid, reqStates := range reqs.Requirements() {
		state := sb.Get(rid)
		if state == status.Undef {
			return fmt.Errorf("invalid requirement: resource '%s' does not exist (syntax: <rid>(<state>[,<state])", rid)
		}
		r.Log().Infof("action %s on resource %s requires %s in states (%s), currently is %s", props.Name, r.RID(), rid, reqStates, state)
		if reqStates.Has(state) {
			continue // requirement met
		}
		deadline, ok := ctx.Deadline()
		var timeout time.Duration
		if ok {
			timeout = deadline.Sub(time.Now())
		} else {
			timeout = time.Minute
		}
		switch props.Name {
		case "start", "stop", "provision", "unprovision", "deploy", "purge":
			r.Log().Infof("requirement not met yet. wait %s", timeout.Round(time.Second))
			state = sb.Wait(rid, timeout)
			if reqStates.Has(state) {
				continue // requirement met
			}
		}
		return fmt.Errorf("%w: action %s on resource %s requires %s in states (%s), but is %s", ErrActionReqNotMet, props.Name, r.RID(), rid, reqStates, state)
	}
	// all requirements met. flag a status transition as pending in the bus.
	sb.Pending(r.RID())
	return nil
}

// Boot deactivates a resource when the node is rebooted
func Boot(ctx context.Context, r Driver) error {
	defer EvalStatus(ctx, r)
	return boot(ctx, r)
}

// Run calls Run() if the resource is a Runner
func Run(ctx context.Context, r Driver) error {
	runner, ok := r.(Runner)
	if !ok {
		return ErrActionNotSupported
	}
	defer EvalStatus(ctx, r)
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := checkRequires(ctx, r); err != nil {
		return fmt.Errorf("run requires: %w", err)
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Pre, trigger.Run); err != nil {
		return fmt.Errorf("pre run trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Pre, trigger.Run); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %d)", err, exitCode(err))
	}
	if err := runner.Run(ctx); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Post, trigger.Run); err != nil {
		return fmt.Errorf("post run trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Post, trigger.Run); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %d)", err, exitCode(err))
	}
	return nil
}

// PRStop deactivates a resource interfacer S3GPR
func PRStop(ctx context.Context, r Driver) error {
	defer EvalStatus(ctx, r)
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := checkRequires(ctx, r); err != nil {
		return fmt.Errorf("start requires: %w", err)
	}
	if err := SCSIPersistentReservationStop(ctx, r); err != nil {
		return err
	}
	return nil
}

// PRStart activates a resource interfacer S3GPR
func PRStart(ctx context.Context, r Driver) error {
	defer EvalStatus(ctx, r)
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := checkRequires(ctx, r); err != nil {
		return fmt.Errorf("start requires: %w", err)
	}
	if err := SCSIPersistentReservationStart(ctx, r); err != nil {
		return err
	}
	return nil
}

// StartStandby activates a resource interfacer
func StartStandby(ctx context.Context, r Driver) error {
	var (
		i  any = r
		fn func(context.Context) error
	)
	if s, ok := i.(startstandbyer); ok {
		fn = s.StartStandby
	} else if s, ok := i.(starter); ok {
		fn = s.Start
	} else {
		return ErrActionNotSupported
	}
	if err := removeStopped(r); err != nil {
		return err
	}
	if !r.IsStandby() {
		return nil
	}
	defer EvalStatus(ctx, r)
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := checkRequires(ctx, r); err != nil {
		return fmt.Errorf("start requires: %w", err)
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Pre, trigger.Start); err != nil {
		return fmt.Errorf("pre start trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Pre, trigger.Start); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %d)", err, exitCode(err))
	}
	if err := SCSIPersistentReservationStart(ctx, r); err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		return fmt.Errorf("start standby: %w", err)
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Post, trigger.Start); err != nil {
		return fmt.Errorf("post start trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Post, trigger.Start); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %d)", err, exitCode(err))
	}
	return nil
}

// Start activates a resource interfacer
func Start(ctx context.Context, r Driver) error {
	var i any = r
	s, ok := i.(starter)
	if !ok {
		return ErrActionNotSupported
	}
	if err := removeStopped(r); err != nil {
		return err
	}
	defer EvalStatus(ctx, r)
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := checkRequires(ctx, r); err != nil {
		return fmt.Errorf("start requires: %w", err)
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Pre, trigger.Start); err != nil {
		return fmt.Errorf("pre start trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Pre, trigger.Start); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %s)", err, exitCode(err))
	}
	if err := SCSIPersistentReservationStart(ctx, r); err != nil {
		return err
	}
	if err := s.Start(ctx); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Post, trigger.Start); err != nil {
		return fmt.Errorf("post start trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Post, trigger.Start); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %s)", err, exitCode(err))
	}
	return nil
}

// Resync execute the resource Resync function, if implemented by the driver.
func Resync(ctx context.Context, r Driver) error {
	var i any = r
	s, ok := i.(resyncer)
	if !ok {
		return ErrActionNotSupported
	}
	defer EvalStatus(ctx, r)
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := s.Resync(ctx); err != nil {
		return err
	}
	return nil
}

// Full execute the resource Update function, if implemented by the driver.
func Full(ctx context.Context, r Driver) error {
	var i any = r
	s, ok := i.(fuller)
	if !ok {
		return ErrActionNotSupported
	}
	defer EvalStatus(ctx, r)
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := s.Full(ctx); err != nil {
		return err
	}
	return nil
}

// Update execute the resource Update function, if implemented by the driver.
func Update(ctx context.Context, r Driver) error {
	var i any = r
	s, ok := i.(updater)
	if !ok {
		return ErrActionNotSupported
	}
	defer EvalStatus(ctx, r)
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := s.Update(ctx); err != nil {
		return err
	}
	return nil
}

// Ingest execute the resource Ingest function, if implemented by the driver.
func Ingest(ctx context.Context, r Driver) error {
	var i any = r
	s, ok := i.(ingester)
	if !ok {
		return ErrActionNotSupported
	}
	defer EvalStatus(ctx, r)
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := s.Ingest(ctx); err != nil {
		return err
	}
	return nil
}

// Shutdown deactivates a resource even if standby is true
func Shutdown(ctx context.Context, r Driver) error {
	defer EvalStatus(ctx, r)
	return shutdown(ctx, r)
}

// Stop deactivates a resource
func Stop(ctx context.Context, r Driver) error {
	defer EvalStatus(ctx, r)
	return stop(ctx, r)
}

// boot turns the resource to a state ready for a start after node reboot.
func boot(ctx context.Context, r Driver) error {
	var (
		i  any = r
		fn func(context.Context) error
	)
	if err := removeStopped(r); err != nil {
		return err
	}
	if s, ok := i.(booter); ok {
		fn = s.Boot
	} else {
		return ErrActionNotSupported
	}
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := fn(ctx); err != nil {
		return err
	}
	return nil
}

// shutdown turns the resource to a state ready for a server halt
//
//	call Shutdown if implemented
//	else call Stop
func shutdown(ctx context.Context, r Driver) error {
	var (
		i  any = r
		fn func(context.Context) error
	)
	if s, ok := i.(shutdowner); ok {
		fn = s.Shutdown
	} else if s, ok := i.(stopper); ok {
		fn = s.Stop
	} else {
		return ErrActionNotSupported
	}
	if err := removeStoppedIfNoResourceSelector(ctx, r); err != nil {
		return err
	}
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := checkRequires(ctx, r); err != nil {
		return fmt.Errorf("requires: %w", err)
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Pre, trigger.Shutdown); err != nil {
		return fmt.Errorf("trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Pre, trigger.Shutdown); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %d)", err, exitCode(err))
	}
	if err := createStoppedIfHasResourceSelector(ctx, r); err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		return err
	}
	if err := SCSIPersistentReservationStop(ctx, r); err != nil {
		return err
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Post, trigger.Shutdown); err != nil {
		return fmt.Errorf("trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Post, trigger.Shutdown); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %s)", err, exitCode(err))
	}
	return nil
}

// stop turns the resource to a state ready for a start.
//
//	standby=false => call Stop
//	standby=true  => call StopStandby if implemented, or do nothing
func stop(ctx context.Context, r Driver) error {
	var (
		i  any = r
		fn func(context.Context) error
	)
	if r.IsStandby() && !actioncontext.IsForce(ctx) {
		if s, ok := i.(stopstandbyer); ok {
			fn = s.StopStandby
		} else {
			if err := removeStoppedIfNoResourceSelector(ctx, r); err != nil {
				return err
			}
			r.Log().Infof("skip 'stop' on standby resource (--force to override)")
			return ErrActionNotSupported
		}
	} else {
		if s, ok := i.(stopper); ok {
			fn = s.Stop
		} else {
			return ErrActionNotSupported
		}
	}
	if err := removeStoppedIfNoResourceSelector(ctx, r); err != nil {
		return err
	}
	if r.IsDisabled() || r.IsActionDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	if err := checkRequires(ctx, r); err != nil {
		return fmt.Errorf("requires: %w", err)
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Pre, trigger.Stop); err != nil {
		return fmt.Errorf("trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Pre, trigger.Stop); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %d)", err, exitCode(err))
	}
	if err := createStoppedIfHasResourceSelector(ctx, r); err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		return err
	}
	if err := SCSIPersistentReservationStop(ctx, r); err != nil {
		return err
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Post, trigger.Stop); err != nil {
		return fmt.Errorf("trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Post, trigger.Stop); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %d)", err, exitCode(err))
	}
	return nil
}

// EvalStatus evaluates the status of a resource interfacer
func EvalStatus(ctx context.Context, r Driver) status.T {
	r.StatusLog().Reset()
	s := status.NotApplicable
	if err := r.GetConfigurationError(); err != nil {
		r.StatusLog().Error("%s", err)
		return s
	}
	var tags []string
	if r.IsActionDisabled() {
		tags = append(tags, "actions disabled")
	}
	if r.IsStatusDisabled() {
		tags = append(tags, "status disabled")
	} else if !r.IsDisabled() {
		Setenv(r)
		s = r.Status(ctx)
		prStatus := SCSIPersistentReservationStatus(ctx, r)
		if s == status.NotApplicable {
			s.Add(prStatus)
		}
		if s == status.Up {
			if isStopped, err := IsStopped(r); err != nil {
				r.StatusLog().Error("%s", err)
			} else if isStopped {
				r.StatusLog().Warn("unmanaged start")
			}
		}
		if r.IsStandby() {
			switch {
			case s == status.Up:
				s = status.StandbyUp
			case s == status.Down:
				s = status.StandbyDown
			}
		}
	}
	if tags != nil {
		r.StatusLog().Info("%s", strings.Join(tags, ", "))
	}
	sb := statusbus.FromContext(ctx)
	sb.Post(r.RID(), s, false)
	return s
}

func newSCSIPersistentRerservationHandle(ctx context.Context, r Driver) *scsi.PersistentReservationHandle {
	var i any = r
	o, ok := i.(devReservabler)
	if !ok {
		r.Log().Tracef("resource does not implement reservable disks listing")
		return nil
	}
	if !o.IsSCSIPersistentReservationEnabled() {
		r.Log().Tracef("scsi pr is not enabled")
		return nil
	}
	hdl := scsi.PersistentReservationHandle{
		Key:            o.PersistentReservationKey(),
		Devices:        o.ReservableDevices(ctx),
		NoPreemptAbort: o.IsSCSIPersistentReservationPreemptAbortDisabled(),
		Force:          actioncontext.IsForce(ctx) || env.HasDaemonMonitorOrigin(),
		Log:            r.Log(),
		StatusLogger:   r.StatusLog(),
	}
	return &hdl
}

func SCSIPersistentReservationStop(ctx context.Context, r Driver) error {
	if hdl := newSCSIPersistentRerservationHandle(ctx, r); hdl == nil {
		return nil
	} else {
		return hdl.Stop()
	}
}

// ImportDevices execute the Driver ImportDevices() function if defined.
// Some drivers need to import devices before they can list the
// reservable devices to register. So use this in the start codepath.
func ImportDevices(ctx context.Context, r Driver) error {
	var i any = r
	o, ok := i.(devImporter)
	if !ok {
		r.Log().Tracef("resource does not implement ImportDevices()")
		return nil
	}
	return o.ImportDevices(ctx)
}

func SCSIPersistentReservationStart(ctx context.Context, r Driver) error {
	if err := ImportDevices(ctx, r); err != nil {
		return err
	}

	if hdl := newSCSIPersistentRerservationHandle(ctx, r); hdl == nil {
		return nil
	} else {
		return hdl.Start()
	}
}

func SCSIPersistentReservationStatus(ctx context.Context, r Driver) status.T {
	if hdl := newSCSIPersistentRerservationHandle(ctx, r); hdl == nil {
		return status.NotApplicable
	} else {
		return hdl.Status()
	}
}

// GetStatus returns the resource Status for embedding into the instance.Status.
func GetStatus(ctx context.Context, r Driver) Status {
	// EvalStatus must be called before formatResourceLabel (it uses context,
	// on containers it will set the initial inspect.
	resStatus := EvalStatus(ctx, r)
	return Status{
		Label:  formatResourceLabel(ctx, r),
		Type:   r.Manifest().DriverID.String(),
		Status: resStatus,
		Subset: r.RSubset(),
		Tags:   r.TagSet(),
		Log:    r.StatusLog().Entries(),
		Info:   getStatusInfo(ctx, r),
		Files:  getFiles(ctx, r),

		IsStopped:   r.IsStopped(),
		IsMonitored: r.IsMonitored(),
		IsOptional:  r.IsOptional(),
		IsStandby:   r.IsStandby(),
		IsDisabled:  r.IsDisabled(),
		IsEncap:     r.IsEncap(),

		// keep last because all previous func calls can add entries
		IsProvisioned: getProvisionStatus(ctx, r),
	}
}

func printStatus(ctx context.Context, r Driver) error {
	data := GetStatus(ctx, r)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func printManifest(r Driver) error {
	m := r.Manifest()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(m)
}

func printHelp(r Driver) error {
	fmt.Println(`Environment variables:
  RES_ACTION=start|stop|status|manifest

Stdin:
  json formatted context data
	`)
	return nil
}

// Action calls the resource method set as the RES_ACTION environment variable
func Action(ctx context.Context, r Driver) error {
	action := os.Getenv("RES_ACTION")
	switch action {
	case "status":
		return printStatus(ctx, r)
	case "stop":
		return Stop(ctx, r)
	case "start":
		return Start(ctx, r)
	case "manifest":
		return printManifest(r)
	default:
		return printHelp(r)
	}
}

// SetLoggerForTest can be used to set resource log for testing purpose
func (t *T) SetLoggerForTest(l *plog.Logger) {
	t.log = *l
}

func (t *T) RunningFromLock(intent string) (RunningInfoList, error) {
	var l RunningInfoList
	p := filepath.Join(t.VarDir(), intent)
	lock := flock.New(p, xsession.ID.String(), fcntllock.New)
	meta, err := lock.Probe()
	if err != nil {
		return l, nil
	}
	if meta.SessionID == "" {
		return l, nil
	}
	sessionID, _ := uuid.Parse(meta.SessionID)
	l = append(l, RunningInfo{
		At:        meta.At,
		PID:       meta.PID,
		RID:       t.RID(),
		SessionID: sessionID,
	})
	return l, nil
}

func (t *T) Lock(disable bool, timeout time.Duration, intent string) (func(), error) {
	if disable {
		// --nolock
		return func() {}, nil
	}
	p := filepath.Join(t.VarDir(), intent)
	lock := flock.New(p, xsession.ID.String(), fcntllock.New)
	err := lock.Lock(timeout, intent)
	if err != nil {
		return nil, err
	}
	return func() { _ = lock.UnLock() }, nil
}

func getStatusInfo(ctx context.Context, t Driver) (data map[string]any) {
	if i, ok := t.(StatusInfoer); ok {
		data = i.StatusInfo(ctx)
	} else {
		data = make(map[string]any)
	}
	if i, ok := t.(Scheduler); ok {
		data["sched"] = getStatusInfoSched(i)
	}
	return data
}

func getStatusInfoSched(t Scheduler) map[string]StatusInfoSchedAction {
	data := make(map[string]StatusInfoSchedAction)
	for _, e := range t.Schedules() {
		ad := StatusInfoSchedAction{
			Last: e.LastRunAt,
		}
		data[e.Action] = ad
	}
	return data
}

func (t SCSIPersistentReservation) IsSCSIPersistentReservationPreemptAbortDisabled() bool {
	return t.NoPreemptAbort
}

func (t SCSIPersistentReservation) IsSCSIPersistentReservationEnabled() bool {
	return t.Enabled
}

func (t SCSIPersistentReservation) PersistentReservationKey() string {
	return t.Key
}

func (t *Status) DeepCopy() *Status {
	newValue := Status{}
	if b, err := json.Marshal(t); err != nil {
		return &Status{}
	} else if err := json.Unmarshal(b, &newValue); err == nil {
		return &newValue
	}
	return &Status{}
}

func (t *Status) Unstructured() map[string]any {
	m := map[string]any{
		"label":       t.Label,
		"status":      t.Status,
		"type":        t.Type,
		"provisioned": t.IsProvisioned,
		"monitor":     t.IsMonitored,
		"disable":     t.IsDisabled,
		"optional":    t.IsOptional,
		"encap":       t.IsEncap,
		"standby":     t.IsStandby,
		"stopped":     t.IsStopped,
	}
	if len(t.Log) > 0 {
		m["log"] = t.Log
	}
	if t.Subset != "" {
		m["subset"] = t.Subset
	}
	if len(t.Tags) > 0 {
		m["tags"] = t.Tags
	}
	if len(t.Info) > 0 {
		m["info"] = t.Info
	}
	return m
}

// Has is true if the rid is found running in the Instance Monitor data sent by the daemon.
func (t *RunningInfoList) Has(rid string) bool {
	for _, r := range *t {
		if r.RID == rid {
			return true
		}
	}
	return false
}

func (t *RunningInfoList) LoadRunDir(rid string, runDir runfiles.Dir) error {
	runDirList, err := runDir.List()
	if err != nil {
		return err
	}
	var errs error
	for _, runfileInfo := range runDirList {
		sessionID, err := uuid.ParseBytes(runfileInfo.Content)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("runfile content parsing to uuid failed: %s", err))
		}
		*t = append(*t, RunningInfo{
			At:        runfileInfo.At,
			PID:       runfileInfo.PID,
			RID:       rid,
			SessionID: sessionID,
		})
	}
	return errs
}

func stoppedFlag(r Driver) string {
	return filepath.Join(r.VarDir(), "stopped")
}

func IsStopped(r Driver) (bool, error) {
	path := stoppedFlag(r)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// removeStoppedIfNoResourceSelector removes the flag file preventing resource restarts by the daemon
func removeStoppedIfNoResourceSelector(ctx context.Context, r Driver) error {
	if actioncontext.HasResourceSelector(ctx) {
		return nil
	}
	return removeStopped(r)
}

func removeStopped(r Driver) error {
	path := stoppedFlag(r)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// createStoppedIfHasResourceSelector creates the flag file preventing resource restarts by the daemon
func createStoppedIfHasResourceSelector(ctx context.Context, r Driver) error {
	if !actioncontext.HasResourceSelector(ctx) {
		return nil
	}
	perm := os.FileMode(0o644)
	path := stoppedFlag(r)
	mkfile := func() (*os.File, error) {
		return os.OpenFile(path, os.O_CREATE|os.O_WRONLY, perm)
	}
	file, err := mkfile()
	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), perm); err != nil {
			return err
		}
		file, err = mkfile()
	}
	if err != nil {
		return err
	}
	return file.Close()
}

func getFiles(ctx context.Context, t Driver) Files {
	i, ok := t.(toSyncer)
	if !ok {
		return nil
	}
	_, isIngester := t.(ingester)

	files := make(Files, 0)
	for _, name := range i.ToSync(ctx) {
		mtime := file.ModTime(name)
		checksum, _ := file.MD5(name)
		file := File{
			Name:     name,
			Mtime:    mtime,
			Checksum: fmt.Sprintf("%x", checksum),
			Ingest:   isIngester,
		}
		files = append(files, file)
	}
	return files
}

func (t Files) Lookup(name string) (File, bool) {
	for _, file := range t {
		if file.Name == name {
			return file, true
		}
	}
	return File{}, false
}
