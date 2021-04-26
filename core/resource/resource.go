package resource

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/golang-collections/collections/set"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	// DriverID identifies a driver.
	DriverID struct {
		Group drivergroup.T
		Name  string
	}

	Logger interface {
		Log() *zerolog.Logger
	}

	// Driver exposes what can be done with a resource
	Driver interface {
		Label() string
		Manifest() *manifest.T
		Start() error
		Stop() error
		Status() status.T

		// common
		SetLog(Logger)
		Log() *zerolog.Logger
		SetRID(string)
		ID() *resourceid.T
		RID() string
		RSubset() string
		TagSet() TagSet
		StatusLog() *StatusLog
		IsOptional() bool
		MatchRID(string) bool
		MatchSubset(string) bool
		MatchTag(string) bool
	}

	Aborter interface {
		Abort() bool
	}

	// T is the resource type, embedded in each drivers type
	T struct {
		Driver
		ResourceID *resourceid.T `json:"rid"`
		Subset     string        `json:"subset"`
		Disable    bool          `json:"disable"`
		Optional   bool          `json:"optional"`
		Tags       *set.Set      `json:"tags"`
		statusLog  StatusLog     `json:"-"`
		log        zerolog.Logger
	}

	// ProvisionStatus define if and when the resource became provisioned.
	ProvisionStatus struct {
		Mtime timestamp.T   `json:"mtime,omitempty"`
		State provisioned.T `json:"state,omitempty"`
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

// SetLogger derives a resource specific logger from the passed logger
func (t *T) SetLog(l Logger) {
	t.log = l.Log().With().Str("rid", t.RID()).Logger()
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

// Start activates a resource interfacer
func Start(r Driver) error {
	return r.Start()
}

// Stop deactivates a resource interfacer
func Stop(r Driver) error {
	return r.Stop()
}

// Status evaluates the status of a resource interfacer
func Status(r Driver) status.T {
	return r.Status()
}

// GetExposedStatus returns the resource exposed status data for embedding into the instance status data.
func GetExposedStatus(r Driver) ExposedStatus {
	return ExposedStatus{
		Label:  formatResourceLabel(r),
		Type:   formatResourceType(r),
		Status: Status(r),
		Subset: r.RSubset(),
		Tags:   r.TagSet(),
		Log:    r.StatusLog().Entries(),
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
