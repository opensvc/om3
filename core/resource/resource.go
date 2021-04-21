package resource

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/golang-collections/collections/set"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/converters"
)

type (
	// DriverID identifies a driver.
	DriverID struct {
		Group drivergroup.T
		Name  string
	}

	// Driver exposes what can be done with a resource
	Driver interface {
		Label() string
		Manifest() Manifest
		Start() error
		Stop() error
		Status() status.T

		// common
		SetRID(string)
		ID() *resourceid.T
		RID() string
		RSubset() string
		RLog() *Log
		IsOptional() bool
		String() string
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
		Log        Log           `json:"-"`
	}

	// OutputStatus is the structure representing the resource status,
	// which is embedded in the instance status.
	OutputStatus struct {
		Label  string      `json:"label"`
		Status status.T    `json:"status"`
		Subset string      `json:"subset,omitempty"`
		Type   string      `json:"type"`
		Log    []*LogEntry `json:"log,omitempty"`
	}
)

var genericKeywords = []keywords.Keyword{
	{
		Option:    "disable",
		Scopable:  false,
		Required:  false,
		Converter: converters.Bool,
		Text:      "",
	},
	{
		Option:    "optional",
		Scopable:  true,
		Required:  false,
		Converter: converters.Bool,
		Text:      "",
	},
	{
		Option:    "tags",
		Scopable:  true,
		Required:  false,
		Converter: converters.Set,
		Text:      "A list of tags. Arbitrary tags can be used to limit action scope to resources with a specific tag. Some tags can influence the driver behaviour. For example :c-tag:`noaction` avoids any state changing action from the driver and implies ``optional=true``, :c-tag:`nostatus` forces the status to n/a.",
	},
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

func (t T) String() string {
	return fmt.Sprintf("<Resource %s>", t.ResourceID)
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

// RLog returns a reference to the resource log
func (t *T) RLog() *Log {
	return &t.Log
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
	return t.Tags.Has(s)
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

func printStatus(r Driver) error {
	data := OutputStatus{
		Label:  formatResourceLabel(r),
		Type:   formatResourceType(r),
		Status: Status(r),
		Subset: r.RSubset(),
		Log:    r.RLog().Entries(),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func printManifest(r Driver) error {
	m := r.Manifest()
	m.Keywords = append(m.Keywords, genericKeywords...)
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
