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
	// Interface exposes what can be done with a Resource
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

	Type struct {
		ResourceId string   `json:"rid"`
		Subset     string   `json:"subset"`
		Log        log.Type `json:"-"`
	}

	OutputStatus struct {
		Label  string      `json:"label"`
		Status status.Type `json:"status"`
		Subset string      `json:"subset,omitempty"`
		Type   string      `json:"type"`
		Log    []log.Entry `json:"log,omitempty"`
	}
)

func (r Type) String() string {
	return fmt.Sprintf("<Resource %s>", r.ResourceId)
}

// GetSubset
func (r Type) GetSubset() string {
	return r.Subset
}

func (r *Type) GetLog() *log.Type {
	return &r.Log
}

func ResourceType(r Interface) string {
	m := r.Manifest()
	return fmt.Sprintf("%s.%s", m.Group, m.Name)
}

func ResourceLabel(r Interface) string {
	return fmt.Sprintf("%s %s", ResourceType(r), r.Label())
}

func Start(r Interface) error {
	return r.Start()
}

func Stop(r Interface) error {
	return r.Stop()
}

func Status(r Interface) status.Type {
	return r.Status()
}

func PrintStatus(r Interface) error {
	data := OutputStatus{
		Label:  ResourceLabel(r),
		Type:   ResourceType(r),
		Status: Status(r),
		Subset: r.GetSubset(),
		Log:    r.GetLog().Dump(),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func PrintManifest(r Interface) error {
	m := r.Manifest()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(m)
}

func PrintHelp(r Interface) error {
	fmt.Println(`Environment variables:
  RES_ACTION=start|stop|status|manifest

Stdin:
  json formatted context data
	`)
	return nil
}

func Action(r Interface) error {
	action := os.Getenv("RES_ACTION")
	switch action {
	case "status":
		return PrintStatus(r)
	case "stop":
		return Stop(r)
	case "start":
		return Start(r)
	case "manifest":
		return PrintManifest(r)
	default:
		return PrintHelp(r)
	}
}
