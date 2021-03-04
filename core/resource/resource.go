package resource

import (
	"encoding/json"
	"fmt"
	"os"

	"opensvc.com/opensvc/core/resource/manifest"
	"opensvc.com/opensvc/core/status"
)

type (
	// Interface exposes what can be done with a resource
	Interface interface {
		Label() string
		Manifest() manifest.Type
		Start() error
		Stop() error
		Status() status.Type

		// common
		RID() string
		Subset() string
		Log() *Log
	}

	// Type is the resource type, embedded in each drivers type
	Type struct {
		rid    string
		subset string
		log    Log `json:"-"`
	}

	// OutputStatus is the structure representing the resource status,
	// which is embedded in the instance status.
	OutputStatus struct {
		Label  string      `json:"label"`
		Status status.Type `json:"status"`
		Subset string      `json:"subset,omitempty"`
		Type   string      `json:"type"`
		Log    []*LogEntry `json:"log,omitempty"`
	}
)

func (r Type) String() string {
	return fmt.Sprintf("<Resource %s>", r.rid)
}

// Subset returns the resource subset name
func (r Type) Subset() string {
	return r.subset
}

// Log return a reference to the resource log
func (r *Type) Log() *Log {
	return &r.log
}

// RID return a reference to the resource log
func (r Type) RID() string {
	return r.rid
}

func formatResourceType(r Interface) string {
	m := r.Manifest()
	return fmt.Sprintf("%s.%s", m.Group, m.Name)
}

func formatResourceLabel(r Interface) string {
	return fmt.Sprintf("%s %s", formatResourceType(r), r.Label())
}

// Start activates a resource interfacer
func Start(r Interface) error {
	return r.Start()
}

// Stop deactivates a resource interfacer
func Stop(r Interface) error {
	return r.Stop()
}

// Status evaluates the status of a resource interfacer
func Status(r Interface) status.Type {
	return r.Status()
}

func printStatus(r Interface) error {
	data := OutputStatus{
		Label:  formatResourceLabel(r),
		Type:   formatResourceType(r),
		Status: Status(r),
		Subset: r.Subset(),
		Log:    r.Log().Entries(),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func printManifest(r Interface) error {
	m := r.Manifest()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(m)
}

func printHelp(r Interface) error {
	fmt.Println(`Environment variables:
  RES_ACTION=start|stop|status|manifest

Stdin:
  json formatted context data
	`)
	return nil
}

// Action calls the resource method set as the RES_ACTION environment variable
func Action(r Interface) error {
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
