package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/golang-collections/collections/set"
	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/core/resourcereqs"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/statusbus"
	"opensvc.com/opensvc/core/trigger"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/pg"
	"opensvc.com/opensvc/util/timestamp"
	"opensvc.com/opensvc/util/xsession"
)

type (
	ObjectDriver interface {
		Log() *zerolog.Logger
		VarDir() string
		ResourceByID(string) Driver
		ResourcesByDrivergroups([]driver.Group) Drivers
	}

	Setenver interface {
		Setenv()
	}

	// Driver exposes what can be done with a resource
	Driver interface {
		Label() string
		Manifest() *manifest.T
		Start(context.Context) error
		Stop(context.Context) error
		Status(context.Context) status.T
		Provisioned() (provisioned.T, error)
		Provision(context.Context) error
		Unprovision(context.Context) error

		// common
		Trigger(trigger.Blocking, trigger.Hook, trigger.Action) error
		Log() *zerolog.Logger
		ID() *resourceid.T
		IsOptional() bool
		IsDisabled() bool
		IsStandby() bool
		IsShared() bool
		IsMonitored() bool
		IsStatusDisabled() bool
		RestartCount() int
		MatchRID(string) bool
		MatchSubset(string) bool
		MatchTag(string) bool
		RID() string
		RSubset() string
		GetObjectDriver() ObjectDriver
		SetObject(interface{})
		GetObject() interface{}
		SetRID(string) error
		SetPG(*pg.Config)
		GetPG() *pg.Config
		GetPGID() string
		ApplyPGChain(context.Context) error
		StatusLog() *StatusLog
		TagSet() TagSet
		VarDir() string
		Requires(string) *resourcereqs.T
	}

	// T is the resource type, embedded in each drivers type
	T struct {
		Driver
		ResourceID              *resourceid.T `json:"rid"`
		Subset                  string        `json:"subset"`
		Disable                 bool          `json:"disable"`
		Monitor                 bool          `json:"monitor"`
		Optional                bool          `json:"optional"`
		Standby                 bool          `json:"standby"`
		Shared                  bool          `json:"shared"`
		Restart                 int           `json:"restart"`
		Tags                    *set.Set      `json:"tags"`
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
		log          zerolog.Logger
		object       interface{}
		objectDriver ObjectDriver
		pg           *pg.Config
	}

	// ProvisionStatus define if and when the resource became provisioned.
	ProvisionStatus struct {
		Mtime timestamp.T   `json:"mtime,omitempty"`
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

	// ExposedStatus is the structure representing the resource status,
	// which is embedded in the instance status.
	ExposedStatus struct {
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
		Info map[string]interface{} `json:"info,omitempty"`

		// Restart is the number of restart to be tried before giving up.
		Restart RestartFlag `json:"restart,omitempty"`

		// Tags is a set of words attached to the resource.
		Tags TagSet `json:"tags,omitempty"`
	}

	Hook int

	ExposedStatusInfoSchedAction struct {
		Last timestamp.T `json:"last"`
	}
)

const (
	Pre Hook = iota
	Post
)

var (
	ErrReqNotMet = errors.New("")
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
	remaining := restart - retries
	switch {
	case restart <= 0:
		return "."
	case remaining < 0:
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

//
// IsOptional returns true if the resource definition contains optional=true.
// An optional resource does not break an object action on error.
//
// Resource having actions disabled are always considered optional, because
// there is nothing we can do to change the state, which would cause
// orchestration loops.
//
func (t T) IsOptional() bool {
	if t.IsActionDisabled() {
		return true
	}
	return t.Optional
}

// IsDisabled returns true if the resource definition container disable=true.
func (t T) IsDisabled() bool {
	return t.Disable
}

// IsStandby returns true if the resource definition container standby=true.
func (t T) IsStandby() bool {
	return t.Standby
}

// IsShared returns true if the resource definition container shared=true.
func (t T) IsShared() bool {
	return t.Shared
}

// IsMonitored returns true if the resource definition container monitor=true.
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

// RSubset returns the resource subset name
func (t T) RSubset() string {
	return t.Subset
}

// StatusLog returns a reference to the resource log
func (t *T) StatusLog() *StatusLog {
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

//
// ApplyPGChain fetches the pg manager from the action context and
// apply the pg configuration to all unconfigured pg on the pg id
// hierarchy (resource=>subset=>object).
//
// The pg manager remembers which pg have been configured to avoid
// doing the config twice.
//
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
			t.log.Info().Msgf("applied %s", run.Config)
		}
	}
	return nil
}

// SetObject holds the useful interface of the parent object of the resource.
func (t *T) SetObject(o interface{}) {
	if _, ok := o.(ObjectDriver); !ok {
		panic("SetObject accepts only ObjectDriver")
	}
	t.object = o
	t.log = t.getLogger()
}

// GetObject returns the object interface set by SetObjectriver upon configure.
func (t T) GetObject() interface{} {
	return t.object
}

// GetObjectDriver returns the object driver interface of the object set by SetObject upon configure.
func (t *T) GetObjectDriver() ObjectDriver {
	return t.object.(ObjectDriver)
}

func (t *T) getLogger() zerolog.Logger {
	l := t.object.(ObjectDriver).Log().With().Stringer("rid", t.ResourceID)
	if t.Subset != "" {
		l = l.Str("rs", t.Subset)
	}
	return l.Logger()
}

// Log returns the resource logger
func (t *T) Log() *zerolog.Logger {
	return &t.log
}

//
// MatchRID returns true if:
//
// * the pattern is a just a drivergroup name and this name matches this resource's drivergroup
//   ex: fs#1 matches fs
// * the pattern is a fully qualified resourceid, and its string representation equals the
//   pattern.
//   ex: fs#1 matches fs#1
//
func (t T) MatchRID(s string) bool {
	rid, err := resourceid.Parse(s)
	if err != nil {
		return false
	}
	if !rid.DriverGroup().IsValid() {
		return false
	}
	if rid.Index() == "" {
		// ex: fs#1 matches fs
		return t.ResourceID.DriverGroup().String() == rid.DriverGroup().String()
	}
	// ex: fs#1 matches fs#1
	return t.ResourceID.String() == s

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
	t.Tags.Do(func(e interface{}) { s = append(s, e.(string)) })
	return s
}

func formatResourceLabel(r Driver) string {
	return fmt.Sprintf("%s %s", r.Manifest().DriverID.Name, r.Label())
}

func (t T) trigger(s string) error {
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

func (t T) Trigger(blocking trigger.Blocking, hook trigger.Hook, action trigger.Action) error {
	var cmd string
	switch trigger.Format(blocking, hook, action) {
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
	t.log.Info().Msgf("trigger %s %s %s: %s", blocking, hook, action, cmd)
	return t.trigger(cmd)
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
		return errors.Wrapf(ErrReqNotMet, "action %s on resource %s requires %s in states (%s), but is %s", props.Name, r.RID(), rid, reqStates, state)
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
		r.Log().Info().Msgf("action %s on resource %s requires %s in states (%s), currently is %s", props.Name, r.RID(), rid, reqStates, state)
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
			r.Log().Info().Msgf("requirement not met yet. wait %s", timeout.Round(time.Second))
			state = sb.Wait(rid, timeout)
			if reqStates.Has(state) {
				continue // requirement met
			}
		}
		return errors.Wrapf(ErrReqNotMet, "action %s on resource %s requires %s in states (%s), but is %s", props.Name, r.RID(), rid, reqStates, state)
	}
	// all requirements met. flag a status transition as pending in the bus.
	sb.Pending(r.RID())
	return nil
}

// Run calls Run() if the resource is a Runner
func Run(ctx context.Context, r Driver) error {
	runner, ok := r.(Runner)
	if !ok {
		return nil
	}
	defer Status(ctx, r)
	if r.IsDisabled() {
		return nil
	}
	Setenv(r)
	if err := checkRequires(ctx, r); err != nil {
		return errors.Wrapf(err, "run requires")
	}
	if err := r.Trigger(trigger.Block, trigger.Pre, trigger.Run); err != nil {
		return errors.Wrapf(err, "pre run trigger")
	}
	if err := r.Trigger(trigger.NoBlock, trigger.Pre, trigger.Run); err != nil {
		r.Log().Warn().Int("exitcode", exitCode(err)).Msgf("trigger: %s", err)
	}
	if err := runner.Run(ctx); err != nil {
		return errors.Wrapf(err, "run")
	}
	if err := r.Trigger(trigger.Block, trigger.Post, trigger.Run); err != nil {
		return errors.Wrapf(err, "post run trigger")
	}
	if err := r.Trigger(trigger.NoBlock, trigger.Post, trigger.Run); err != nil {
		r.Log().Warn().Int("exitcode", exitCode(err)).Msgf("trigger: %s", err)
	}
	return nil
}

// Start activates a resource interfacer
func Start(ctx context.Context, r Driver) error {
	defer Status(ctx, r)
	if r.IsDisabled() {
		return nil
	}
	Setenv(r)
	if err := checkRequires(ctx, r); err != nil {
		return errors.Wrapf(err, "start requires")
	}
	if err := r.Trigger(trigger.Block, trigger.Pre, trigger.Start); err != nil {
		return errors.Wrapf(err, "pre start trigger")
	}
	if err := r.Trigger(trigger.NoBlock, trigger.Pre, trigger.Start); err != nil {
		r.Log().Warn().Int("exitcode", exitCode(err)).Msgf("trigger: %s", err)
	}
	if err := r.Start(ctx); err != nil {
		return errors.Wrapf(err, "start")
	}
	if err := r.Trigger(trigger.Block, trigger.Post, trigger.Start); err != nil {
		return errors.Wrapf(err, "post start trigger")
	}
	if err := r.Trigger(trigger.NoBlock, trigger.Post, trigger.Start); err != nil {
		r.Log().Warn().Int("exitcode", exitCode(err)).Msgf("trigger: %s", err)
	}
	return nil
}

// Resync deactivates a resource interfacer
func Resync(ctx context.Context, r Driver) error {
	var i interface{} = r
	s, ok := i.(resyncer)
	if !ok {
		return nil
	}
	defer Status(ctx, r)
	if r.IsDisabled() {
		return nil
	}
	Setenv(r)
	if err := s.Resync(ctx); err != nil {
		return err
	}
	return nil
}

// Stop deactivates a resource interfacer
func Stop(ctx context.Context, r Driver) error {
	defer Status(ctx, r)
	if r.IsDisabled() {
		return nil
	}
	Setenv(r)
	if err := checkRequires(ctx, r); err != nil {
		return errors.Wrapf(err, "requires")
	}
	if err := r.Trigger(trigger.Block, trigger.Pre, trigger.Stop); err != nil {
		return errors.Wrapf(err, "trigger")
	}
	if err := r.Trigger(trigger.NoBlock, trigger.Pre, trigger.Stop); err != nil {
		r.Log().Warn().Int("exitcode", exitCode(err)).Msgf("trigger: %s", err)
	}
	if err := r.Stop(ctx); err != nil {
		return err
	}
	if err := r.Trigger(trigger.Block, trigger.Post, trigger.Stop); err != nil {
		return errors.Wrapf(err, "trigger")
	}
	if err := r.Trigger(trigger.NoBlock, trigger.Post, trigger.Stop); err != nil {
		r.Log().Warn().Int("exitcode", exitCode(err)).Msgf("trigger: %s", err)
	}
	return nil
}

// Status evaluates the status of a resource interfacer
func Status(ctx context.Context, r Driver) status.T {
	r.StatusLog().Reset()
	if r.IsStatusDisabled() {
		r.StatusLog().Info("nostatus")
	}
	s := status.NotApplicable
	if !r.IsDisabled() {
		Setenv(r)
		s = r.Status(ctx)
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

// GetExposedStatus returns the resource exposed status data for embedding into the instance status data.
func GetExposedStatus(ctx context.Context, r Driver) ExposedStatus {
	return ExposedStatus{
		Label:       formatResourceLabel(r),
		Type:        r.Manifest().DriverID.String(),
		Status:      Status(ctx, r),
		Subset:      r.RSubset(),
		Tags:        r.TagSet(),
		Log:         r.StatusLog().Entries(),
		Provisioned: getProvisionStatus(r),
		Info:        exposedStatusInfo(r),
		Restart:     RestartFlag(r.RestartCount()),
		Optional:    OptionalFlag(r.IsOptional()),
		Standby:     StandbyFlag(r.IsStandby()),
		Disable:     DisableFlag(r.IsDisabled()),
		//Encap:       EncapFlag(r.IsEncap()),
	}
}

func printStatus(ctx context.Context, r Driver) error {
	data := GetExposedStatus(ctx, r)
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
func (t *T) SetLoggerForTest(l zerolog.Logger) {
	t.log = l
}

func (t *T) DoWithLock(disable bool, timeout time.Duration, intent string, f func() error) error {
	if disable {
		// --nolock
		return nil
	}
	p := filepath.Join(t.VarDir(), intent)
	lock := flock.New(p, xsession.ID, fcntllock.New)
	err := lock.Lock(timeout, intent)
	if err != nil {
		return err
	}
	defer func() { _ = lock.UnLock() }()
	return f()
}

func exposedStatusInfo(t Driver) (data map[string]interface{}) {
	if i, ok := t.(StatusInfoer); ok {
		data = i.StatusInfo()
	} else {
		data = make(map[string]interface{})
	}
	if i, ok := t.(Scheduler); ok {
		data["sched"] = exposedStatusInfoSched(i)
	}
	return data
}

func exposedStatusInfoSched(t Scheduler) map[string]ExposedStatusInfoSchedAction {
	data := make(map[string]ExposedStatusInfoSchedAction)
	for _, e := range t.Schedules() {
		ad := ExposedStatusInfoSchedAction{
			Last: e.Last,
		}
		data[e.Action] = ad
	}
	return data
}

func (exposedStatus ExposedStatus) DeepCopy() *ExposedStatus {
	newValue := ExposedStatus{}
	if b, err := json.Marshal(exposedStatus); err != nil {
		return &ExposedStatus{}
	} else if err := json.Unmarshal(b, &newValue); err == nil {
		return &newValue
	}
	return &ExposedStatus{}
}
