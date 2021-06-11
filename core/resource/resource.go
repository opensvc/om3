package resource

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/golang-collections/collections/set"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/trigger"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	// DriverID identifies a driver.
	DriverID struct {
		Group drivergroup.T
		Name  string
	}

	ObjectDriver interface {
		Log() *zerolog.Logger
		VarDir() string
	}

	Setenver interface {
		Setenv()
	}

	// Driver exposes what can be done with a resource
	Driver interface {
		Label() string
		Manifest() *manifest.T
		Start() error
		Stop() error
		Status() status.T
		Provisioned() (provisioned.T, error)
		Provision() error
		Unprovision() error

		// common
		Trigger(trigger.Blocking, trigger.Hook, trigger.Action) error
		Log() *zerolog.Logger
		ID() *resourceid.T
		IsOptional() bool
		IsDisabled() bool
		MatchRID(string) bool
		MatchSubset(string) bool
		MatchTag(string) bool
		RID() string
		RSubset() string
		SetObjectDriver(ObjectDriver)
		GetObjectDriver() ObjectDriver
		SetRID(string)
		StatusLog() *StatusLog
		TagSet() TagSet
		VarDir() string
	}

	Aborter interface {
		Abort() bool
	}

	// T is the resource type, embedded in each drivers type
	T struct {
		Driver
		ResourceID        *resourceid.T `json:"rid"`
		Subset            string        `json:"subset"`
		Disable           bool          `json:"disable"`
		Optional          bool          `json:"optional"`
		Tags              *set.Set      `json:"tags"`
		BlockingPreStart  string
		BlockingPreStop   string
		PreStart          string
		PreStop           string
		BlockingPostStart string
		BlockingPostStop  string
		PostStart         string
		PostStop          string

		statusLog StatusLog
		log       zerolog.Logger
		object    ObjectDriver
	}

	// ProvisionStatus define if and when the resource became provisioned.
	ProvisionStatus struct {
		Mtime timestamp.T   `json:"mtime,omitempty"`
		State provisioned.T `json:"state"`
	}

	// MonitorFlag tells the daemon if it should trigger a monitor action
	// when the resource is not up.
	MonitorFlag bool

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
		ResourceID  resourceid.T      `json:"-"`
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
		Restart int `json:"restart,omitempty"`

		// Tags is a set of words attached to the resource.
		Tags TagSet `json:"tags,omitempty"`
	}

	Hook int
)

const (
	Pre Hook = iota
	Post
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

func (t DriverID) String() string {
	if t.Name == "" {
		return t.Group.String()
	}
	return fmt.Sprintf("%s.%s", t.Group, t.Name)
}

func ParseDriverID(s string) *DriverID {
	l := strings.SplitN(s, ".", 2)
	g := drivergroup.New(l[0])
	return &DriverID{
		Group: g,
		Name:  l[1],
	}
}

func NewDriverID(group drivergroup.T, name string) *DriverID {
	return &DriverID{
		Group: group,
		Name:  name,
	}
}

var drivers = make(map[DriverID]func() Driver)

func Register(group drivergroup.T, name string, f func() Driver) {
	driverID := NewDriverID(group, name)
	drivers[*driverID] = f
}

func (t DriverID) NewResourceFunc() func() Driver {
	drv, ok := drivers[t]
	if !ok {
		return nil
	}
	return drv
}

//
// IsOptional returns true if the resource definition contains optional=true.
// An optional resource does not break an object action on error.
//
func (t T) IsOptional() bool {
	return t.Optional
}

// IsDisabled returns true if the resource definition container disable=true.
func (t T) IsDisabled() bool {
	return t.Disable
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
func (t *T) SetRID(v string) {
	t.ResourceID = resourceid.Parse(v)
}

// SetObjectDriver holds the useful interface of the parent object of the resource.
func (t *T) SetObjectDriver(o ObjectDriver) {
	t.object = o
	t.log = t.getLogger()
}

// GetObjectDriver returns the object driver interface set by SetObjectDriver upon configure.
func (t *T) GetObjectDriver() ObjectDriver {
	return t.object
}

func (t *T) getLogger() zerolog.Logger {
	l := t.object.Log().With().Stringer("rid", t.ResourceID)
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
	rid := resourceid.Parse(s)
	if !rid.DriverGroup().IsValid() {
		return false
	}
	if rid.Name == "" {
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

func formatResourceType(r Driver) string {
	m := r.Manifest()
	return fmt.Sprintf("%s.%s", m.Group, m.Name)
}

func formatResourceLabel(r Driver) string {
	return fmt.Sprintf("%s %s", formatResourceType(r), r.Label())
}

func (t T) trigger(s string) error {
	cmd, err := command.CommandFromString(s)
	if err != nil {
		return err
	}
	if cmd == nil {
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	bufout := bufio.NewReader(stdout)
	buferr := bufio.NewReader(stderr)
	if err := cmd.Start(); err != nil {
		return err
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for {
			line, _, _ := bufout.ReadLine()
			if len(line) == 0 {
				break
			}
			t.log.Info().Msg(string(line))
		}
		wg.Done()
	}()
	go func() {
		for {
			line, _, _ := buferr.ReadLine()
			if len(line) == 0 {
				break
			}
			t.log.Error().Msg(string(line))
		}
		wg.Done()
	}()
	wg.Wait()
	return cmd.Wait()
}

func (t T) Trigger(blocking trigger.Blocking, hook trigger.Hook, action trigger.Action) error {
	var cmd string
	switch {
	//
	case action == trigger.Start && hook == trigger.Pre && blocking == trigger.Block:
		cmd = t.BlockingPreStart
	case action == trigger.Start && hook == trigger.Pre && blocking == trigger.NoBlock:
		cmd = t.PreStart
	case action == trigger.Start && hook == trigger.Post && blocking == trigger.Block:
		cmd = t.BlockingPostStart
	case action == trigger.Start && hook == trigger.Post && blocking == trigger.NoBlock:
		cmd = t.PostStart
	//
	case action == trigger.Stop && hook == trigger.Pre && blocking == trigger.Block:
		cmd = t.BlockingPreStop
	case action == trigger.Stop && hook == trigger.Pre && blocking == trigger.NoBlock:
		cmd = t.PreStop
	case action == trigger.Stop && hook == trigger.Post && blocking == trigger.Block:
		cmd = t.BlockingPostStop
	case action == trigger.Stop && hook == trigger.Post && blocking == trigger.NoBlock:
		cmd = t.PostStop
	default:
		return nil
	}
	if cmd == "" {
		return nil
	}
	t.log.Info().Msgf("trigger %s %s %s: %s", blocking, hook, action, cmd)
	return t.trigger(cmd)
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

// Start activates a resource interfacer
func Start(r Driver) error {
	Setenv(r)
	if err := r.Trigger(trigger.Block, trigger.Pre, trigger.Start); err != nil {
		return errors.Wrapf(err, "trigger")
	}
	if err := r.Trigger(trigger.NoBlock, trigger.Pre, trigger.Start); err != nil {
		r.Log().Warn().Int("exitcode", exitCode(err)).Msgf("trigger: %s", err)
	}
	if err := r.Start(); err != nil {
		return err
	}
	if err := r.Trigger(trigger.Block, trigger.Post, trigger.Start); err != nil {
		return errors.Wrapf(err, "trigger")
	}
	if err := r.Trigger(trigger.NoBlock, trigger.Post, trigger.Start); err != nil {
		r.Log().Warn().Int("exitcode", exitCode(err)).Msgf("trigger: %s", err)
	}
	return nil
}

// Stop deactivates a resource interfacer
func Stop(r Driver) error {
	Setenv(r)
	if err := r.Trigger(trigger.Block, trigger.Pre, trigger.Stop); err != nil {
		return errors.Wrapf(err, "trigger")
	}
	if err := r.Trigger(trigger.NoBlock, trigger.Pre, trigger.Stop); err != nil {
		r.Log().Warn().Int("exitcode", exitCode(err)).Msgf("trigger: %s", err)
	}
	if err := r.Stop(); err != nil {
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
func Status(r Driver) status.T {
	Setenv(r)
	return r.Status()
}

// GetExposedStatus returns the resource exposed status data for embedding into the instance status data.
func GetExposedStatus(r Driver) ExposedStatus {
	return ExposedStatus{
		Label:       formatResourceLabel(r),
		Type:        formatResourceType(r),
		Status:      Status(r),
		Subset:      r.RSubset(),
		Tags:        r.TagSet(),
		Log:         r.StatusLog().Entries(),
		Provisioned: getProvisionStatus(r),
	}
}

func printStatus(r Driver) error {
	data := GetExposedStatus(r)
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
func Action(r Driver) error {
	action := os.Getenv("RES_ACTION")
	switch action {
	case "status":
		return printStatus(r)
	case "stop":
		return Stop(r)
	case "start":
		return Start(r)
	case "manifest":
		return printManifest(r)
	default:
		return printHelp(r)
	}
}
