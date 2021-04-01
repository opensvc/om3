package resource

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"opensvc.com/opensvc/core/status"
)

type (
	// DriverID identifies a driver.
	DriverID struct {
		Group string
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
		RID() string
		RSubset() string
		RLog() *Log
	}

	// T is the resource type, embedded in each drivers type
	T struct {
		ResourceID string `json:"rid"`
		Subset     string `json:"subset"`
		Disable    bool   `json:"disable"`
		Log        Log    `json:"-"`
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

func (t DriverID) String() string {
	if t.Name == "" {
		return t.Group
	}
	return fmt.Sprintf("%s.%s", t.Group, t.Name)
}

func ParseDriverID(s string) *DriverID {
	l := strings.SplitN(s, ".", 2)
	return &DriverID{
		Group: l[0],
		Name:  l[1],
	}
}

func NewDriverID(group string, name string) *DriverID {
	return &DriverID{
		Group: group,
		Name:  name,
	}
}

var drivers = make(map[string]func() Driver)

func Register(group string, name string, f func() Driver) {
	driverId := NewDriverID(group, name)
	drivers[driverId.String()] = f
}

func (t T) String() string {
	return fmt.Sprintf("<Resource %s>", t.ResourceID)
}

// RSubset returns the resource subset name
func (t T) RSubset() string {
	return t.Subset
}

// RLog return a reference to the resource log
func (t *T) RLog() *Log {
	return &t.Log
}

// RID return a reference to the resource log
func (t T) RID() string {
	return t.ResourceID
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
