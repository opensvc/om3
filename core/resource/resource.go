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
	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/colorstatus"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/core/resourcereqs"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/statusbus"
	"github.com/opensvc/om3/core/trigger"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/pg"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/progress"
	"github.com/opensvc/om3/util/scsi"
	"github.com/opensvc/om3/util/xsession"
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
		Reset()
		Entries() []*StatusLogEntry
	}

	// Driver exposes what can be done with a resource
	Driver interface {
		Provisioned() (provisioned.T, error)
		Provision(context.Context) error
		Unprovision(context.Context) error

		// common
		ApplyPGChain(context.Context) error
		GetObject() any
		GetPG() *pg.Config
		GetPGID() string
		GetRestartDelay() time.Duration
		ID() *resourceid.T
		IsDisabled() bool
		IsEncap() bool
		IsMonitored() bool
		IsOptional() bool
		IsProvisionDisabled() bool
		IsUnprovisionDisabled() bool
		IsShared() bool
		IsStandby() bool
		IsStatusDisabled() bool
		Label() string
		Log() *plog.Logger
		Manifest() *manifest.T
		MatchRID(string) bool
		MatchSubset(string) bool
		MatchTag(string) bool
		Progress(context.Context, ...any)
		ProgressKey() []string
		Requires(string) *resourcereqs.T
		RestartCount() int
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
		Restart                 int
		RestartDelay            *time.Duration
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

		statusLog    StatusLog
		log          plog.Logger
		object       any
		objectDriver ObjectDriver
		pg           *pg.Config
	}

	// devReservabler is an interface implemented by resource drivers that want the core resource
	// to handle SCSI persistent reservation on a list of devices.
	devReservabler interface {
		// ReservableDevices must be implement by every driver that wants SCSI PR.
		ReservableDevices() device.L

		// IsSCSIPersistentReservationPreemptAbortDisabled is exposing the resource no_preempt_abort keyword value.
		IsSCSIPersistentReservationPreemptAbortDisabled() bool

		// IsSCSIPersistentReservationEnabled is exposing the scsireserv resource keyword value.
		IsSCSIPersistentReservationEnabled() bool

		// PersistentReservationKey is exposing the prkey resource keyword value.
		PersistentReservationKey() string
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

	// MonitorFlag tells the daemon if it should trigger a monitor action
	// when the resource is not up.
	MonitorFlag bool

	// RestartFlag is the number of times the monitor will try restarting a
	// resource gone down in a well-known started instance.
	RestartFlag int

	// DisableFlag hints the resource ignores all state transition actions
	DisableFlag bool

	// OptionalFlag makes this resource status aggregated into Overall
	// instead of Avail instance status. Errors in optional resource don't stop
	// a state transition action.
	OptionalFlag bool

	// EncapFlag indicates that the resource is handled by the encapsulated
	// agents, and ignored at the hypervisor level.
	EncapFlag bool

	// StandbyFlag tells the daemon this resource should always be up,
	// even after a stop state transition action.
	StandbyFlag bool

	// TagSet is the list of unique tag names found in the resource definition.
	TagSet []string

	// Status is the structure representing the resource status,
	// which is embedded in the instance status.
	Status struct {
		ResourceID  *resourceid.T     `json:"-"`
		Label       string            `json:"label"`
		Log         []*StatusLogEntry `json:"log,omitempty"`
		Status      status.T          `json:"status"`
		Type        string            `json:"type"`
		Provisioned ProvisionStatus   `json:"provisioned,omitempty"`
		Monitor     MonitorFlag       `json:"monitor,omitempty"`
		Disable     DisableFlag       `json:"disable,omitempty"`
		Optional    OptionalFlag      `json:"optional,omitempty"`
		Encap       EncapFlag         `json:"encap,omitempty"`
		Standby     StandbyFlag       `json:"standby,omitempty"`

		// Subset is the name of the subset this resource is assigned to.
		Subset string `json:"subset,omitempty"`

		// Info is a list of key-value pairs providing interesting information to
		// collect site-wide about this resource.
		Info map[string]any `json:"info,omitempty"`

		// Restart is the number of restart to be tried before giving up.
		Restart RestartFlag `json:"restart,omitempty"`

		// Tags is a set of words attached to the resource.
		Tags TagSet `json:"tags,omitempty"`
	}

	Hook int

	StatusInfoSchedAction struct {
		Last time.Time `json:"last"`
	}

	// ScheduleOptions contains the information needed by the object to create a
	// schedule.Entry to append to the object's schedule.Table.
	ScheduleOptions struct {
		Action             string
		Option             string
		Base               string
		RequireCollector   bool
		RequireProvisioned bool
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
)

// FlagString returns a one character representation of the type instance.
func (t MonitorFlag) FlagString() string {
	if t {
		return "M"
	}
	return "."
}

// FlagString returns a one character representation of the type instance.
func (t DisableFlag) FlagString() string {
	if t {
		return "D"
	}
	return "."
}

// FlagString returns a one character representation of the type instance.
func (t RestartFlag) FlagString(retries int) string {
	restart := int(t)
	remaining := retries
	switch {
	case restart <= 0:
		return "."
	case remaining <= 0:
		return "0"
	case remaining < 10:
		return fmt.Sprintf("%d", remaining)
	default:
		return "+"
	}
}

// FlagString returns a one character representation of the type instance.
func (t OptionalFlag) FlagString() string {
	if t {
		return "O"
	}
	return "."
}

// FlagString returns a one character representation of the type instance.
func (t EncapFlag) FlagString() string {
	if t {
		return "E"
	}
	return "."
}

// FlagString returns a one character representation of the type instance.
func (t StandbyFlag) FlagString() string {
	if t {
		return "S"
	}
	return "."
}

func NewResourceFunc(drvID driver.ID) func() Driver {
	i := driver.Get(drvID)
	if i == nil {
		return nil
	}
	if a, ok := i.(func() Driver); ok {
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
func (t T) IsOptional() bool {
	if t.IsActionDisabled() {
		return true
	}
	return t.Optional
}

// IsEncap returns true if the resource definition contains encap=true.
func (t T) IsEncap() bool {
	return t.Encap || t.Tags.Has("encap")
}

// IsDisabled returns true if the resource definition contains disable=true.
func (t T) IsDisabled() bool {
	return t.Disable
}

// IsProvisionDisabled returns true if the resource definition contains provision=false.
func (t T) IsProvisionDisabled() bool {
	return !t.EnableProvision
}

// IsUnprovisionDisabled returns true if the resource definition contains unprovision=false.
func (t T) IsUnprovisionDisabled() bool {
	return !t.EnableUnprovision
}

// IsStandby returns true if the resource definition contains standby=true.
func (t T) IsStandby() bool {
	return t.Standby
}

// IsShared returns true if the resource definition contains shared=true.
func (t T) IsShared() bool {
	return t.Shared
}

// IsMonitored returns true if the resource definition contains monitor=true.
func (t T) IsMonitored() bool {
	return t.Monitor
}

// IsStatusDisabled returns true if the resource definition contains tag=nostatus ...
// In this case, the resource status is always n/a
func (t T) IsStatusDisabled() bool {
	return t.MatchTag("nostatus")
}

// IsActionDisabled returns true if the resource definition contains tag=noaction ...
// In this case, the resource actions like stop and start are skipped.
func (t T) IsActionDisabled() bool {
	return t.MatchTag("noaction")
}

// RestartCount returns the value of the Restart field
func (t T) RestartCount() int {
	return t.Restart
}

// GetRestartDelay returns the duration between 2 restarts
func (t T) GetRestartDelay() time.Duration {
	if t.RestartDelay == nil {
		return 500 * time.Millisecond
	}
	return *t.RestartDelay
}

// RSubset returns the resource subset name
func (t T) RSubset() string {
	return t.Subset
}

// StatusLog returns a reference to the resource log
func (t *T) StatusLog() StatusLogger {
	return &t.statusLog
}

// RID returns the string representation of the resource id
func (t T) RID() string {
	return t.ResourceID.String()
}

// ID returns the resource id struct
func (t T) ID() *resourceid.T {
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
	for _, run := range mgr.Apply(t.GetPGID()) {
		if !run.Changed {
			continue
		}
		if run.Err != nil {
			return run.Err
		} else {
			t.Log().Infof("applied %s", run.Config)
		}
	}
	return nil
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
func (t T) GetObject() any {
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
func (t T) MatchRID(s string) bool {
	return t.ResourceID.Match(s)

}

// MatchSubset returns true if the resource subset equals the pattern.
func (t T) MatchSubset(s string) bool {
	return t.Subset == s
}

// MatchTag returns true if one of the resource tags equals the pattern.
func (t T) MatchTag(s string) bool {
	if t.Tags == nil {
		return false
	}
	return t.Tags.Has(s)
}

func (t T) TagSet() TagSet {
	s := make(TagSet, 0)
	t.Tags.Do(func(e any) { s = append(s, e.(string)) })
	return s
}

func formatResourceLabel(r Driver) string {
	name := r.Manifest().DriverID.Name
	if name == "" {
		return r.Label()
	} else {
		return strings.Join([]string{name, r.Label()}, " ")
	}
}

func (t T) trigger(ctx context.Context, s string) error {
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
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel))
	return cmd.Run()
}

func (t T) Trigger(ctx context.Context, blocking trigger.Blocking, hook trigger.Hook, action trigger.Action) error {
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
	t.Progress(ctx, "▶ "+hookID)
	return t.trigger(ctx, cmd)
}

func (t T) Requires(action string) *resourcereqs.T {
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

func StatusCheckRequires(ctx context.Context, r Driver) error {
	props := actioncontext.Props(ctx)
	reqs := r.Requires(props.Name)
	sb := statusbus.FromContext(ctx)
	for rid, reqStates := range reqs.Requirements() {
		state := sb.Get(rid)
		if state == status.Undef {
			return fmt.Errorf("invalid requirement: resource '%s' does not exist (syntax: <rid>(<state>[,<state])", rid)
		}
		if reqStates.Has(state) {
			continue // requirement met
		}
		return fmt.Errorf("%w: action %s on resource %s requires %s in states (%s), but is %s", ErrActionReqNotMet, props.Name, r.RID(), rid, reqStates, state)
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
	if r.IsDisabled() {
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
	r.Progress(ctx, "▶ run")
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
	if r.IsDisabled() {
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
	if r.IsDisabled() {
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
	if !r.IsStandby() {
		return nil
	}
	defer EvalStatus(ctx, r)
	if r.IsDisabled() {
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
	r.Progress(ctx, "▶ start standby")
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
	defer EvalStatus(ctx, r)
	if r.IsDisabled() {
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
	r.Progress(ctx, "▶ start")
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
	if r.IsDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	r.Progress(ctx, "▶ resync")
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
	if r.IsDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	r.Progress(ctx, "▶ full")
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
	if r.IsDisabled() {
		return ErrDisabled
	}
	r.Progress(ctx, "▶ update")
	Setenv(r)
	if err := s.Update(ctx); err != nil {
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
		progressAction string
		i              any = r
		fn             func(context.Context) error
	)
	if s, ok := i.(booter); ok {
		fn = s.Boot
		progressAction = "boot"
	} else {
		return ErrActionNotSupported
	}
	if r.IsDisabled() {
		return ErrDisabled
	}
	Setenv(r)
	r.Progress(ctx, "▶ "+progressAction)
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
	if r.IsDisabled() {
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
	r.Progress(ctx, "▶ shutdown")
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
		progressAction string
		i              any = r
		fn             func(context.Context) error
	)
	if r.IsStandby() {
		if s, ok := i.(stopstandbyer); ok {
			fn = s.StopStandby
			progressAction = "standby"
		} else {
			return ErrActionNotSupported
		}
	} else {
		if s, ok := i.(stopper); ok {
			fn = s.Stop
			progressAction = "stop"
		} else {
			return ErrActionNotSupported
		}
	}
	if r.IsDisabled() {
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
	r.Progress(ctx, "▶ "+progressAction)
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
	if r.IsStatusDisabled() {
		r.StatusLog().Info("nostatus")
	}
	s := status.NotApplicable
	if !r.IsDisabled() {
		Setenv(r)
		s = r.Status(ctx)
		prStatus := SCSIPersistentReservationStatus(r)
		if s == status.NotApplicable {
			s.Add(prStatus)
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

	sb := statusbus.FromContext(ctx)
	sb.Post(r.RID(), s, false)
	return s
}

func newSCSIPersistentRerservationHandle(r Driver) *scsi.PersistentReservationHandle {
	var i any = r
	o, ok := i.(devReservabler)
	if !ok {
		r.Log().Debugf("resource does not implement reservable disks listing")
		return nil
	}
	if !o.IsSCSIPersistentReservationEnabled() {
		r.Log().Debugf("scsi pr is not enabled")
		return nil
	}
	hdl := scsi.PersistentReservationHandle{
		Key:            o.PersistentReservationKey(),
		Devices:        o.ReservableDevices(),
		NoPreemptAbort: o.IsSCSIPersistentReservationPreemptAbortDisabled(),
		Log:            r.Log(),
		StatusLogger:   r.StatusLog(),
	}
	return &hdl
}

func SCSIPersistentReservationStop(ctx context.Context, r Driver) error {
	if hdl := newSCSIPersistentRerservationHandle(r); hdl == nil {
		return nil
	} else {
		r.Progress(ctx, "▶ prstop")
		return hdl.Stop()
	}
}

func SCSIPersistentReservationStart(ctx context.Context, r Driver) error {
	if hdl := newSCSIPersistentRerservationHandle(r); hdl == nil {
		return nil
	} else {
		r.Progress(ctx, "▶ prstart")
		return hdl.Start()
	}
}

func SCSIPersistentReservationStatus(r Driver) status.T {
	if hdl := newSCSIPersistentRerservationHandle(r); hdl == nil {
		return status.NotApplicable
	} else {
		return hdl.Status()
	}
}

// GetStatus returns the resource Status for embedding into the instance.Status.
func GetStatus(ctx context.Context, r Driver) Status {
	return Status{
		Label:       formatResourceLabel(r),
		Type:        r.Manifest().DriverID.String(),
		Status:      EvalStatus(ctx, r),
		Subset:      r.RSubset(),
		Tags:        r.TagSet(),
		Log:         r.StatusLog().Entries(),
		Provisioned: getProvisionStatus(r),
		Info:        getStatusInfo(r),
		Restart:     RestartFlag(r.RestartCount()),
		Optional:    OptionalFlag(r.IsOptional()),
		Standby:     StandbyFlag(r.IsStandby()),
		Disable:     DisableFlag(r.IsDisabled()),
		Encap:       EncapFlag(r.IsEncap()),
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

func getStatusInfo(t Driver) (data map[string]any) {
	if i, ok := t.(StatusInfoer); ok {
		data = i.StatusInfo()
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
	if t.Key != "" {
		return t.Key
	}
	if nodePRKey := rawconfig.GetNodeSection().PRKey; nodePRKey != "" {
		return scsi.StripPRKey(nodePRKey)
	}
	return ""
}

func (t *T) ProgressKey() []string {
	p := rawconfig.Colorize.Bold(naming.PathOf(t.object).String())
	return []string{p, t.RID()}
}

// progressMsg prepends the last known colored status or the resource
func (t *T) progressMsg(ctx context.Context, msg *string) []any {
	sb := statusbus.FromContext(ctx)
	rid := t.RID()
	first := colorstatus.Sprint(sb.First(rid), rawconfig.Colorize)
	last := colorstatus.Sprint(sb.Get(rid), rawconfig.Colorize)
	return []any{first, last, msg}
}

func (t *T) Progress(ctx context.Context, cols ...any) {
	if view := progress.ViewFromContext(ctx); view != nil {
		key := t.ProgressKey()
		view.Info(key, cols)
	}
}

func (t Status) DeepCopy() *Status {
	newValue := Status{}
	if b, err := json.Marshal(t); err != nil {
		return &Status{}
	} else if err := json.Unmarshal(b, &newValue); err == nil {
		return &newValue
	}
	return &Status{}
}

func (t Status) Unstructured() map[string]any {
	m := map[string]any{
		"label":       t.Label,
		"status":      t.Status,
		"type":        t.Type,
		"provisioned": t.Provisioned,
		"monitor":     t.Monitor,
		"disable":     t.Disable,
		"optional":    t.Optional,
		"encap":       t.Encap,
		"restart":     t.Restart,
		"standby":     t.Standby,
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
