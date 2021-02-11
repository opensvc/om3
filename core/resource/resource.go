package resource

import (
	"encoding/json"
	"fmt"
	"os"

	"opensvc.com/opensvc/core/resource/log"
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
		GetSubset() string
		GetLog() *log.Type
	}

	// Type is the resource type, embedded in each drivers type
	Type struct {
		RID    string   `json:"rid"`
		Subset string   `json:"subset"`
		Log    log.Type `json:"-"`
	}

	// OutputStatus is the structure representing the resource status,
	// which is embedded in the instance status.
	OutputStatus struct {
		Label  string      `json:"label"`
		Status status.Type `json:"status"`
		Subset string      `json:"subset,omitempty"`
		Type   string      `json:"type"`
		Log    []log.Entry `json:"log,omitempty"`
	}
)

func (r Type) String() string {
	return fmt.Sprintf("<Resource %s>", r.RID)
}

// GetSubset returns the resource subset name
func (r Type) GetSubset() string {
	return r.Subset
}

// GetLog return a reference to the resource log
func (r *Type) GetLog() *log.Type {
	return &r.Log
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
		Subset: r.GetSubset(),
		Log:    r.GetLog().Dump(),
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
