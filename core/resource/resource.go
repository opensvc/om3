package resource

import (
	"encoding/json"
	"fmt"
	"os"

	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/status"
)

type (
	ResourceInterface interface {
		Label()		string
		Manifest()	ManifestType
		Start()		error
		Stop()		error
		Status()	status.StatusType

		// common
		GetSubset()	string
		GetLog()	*LogType
	}

	ManifestType struct {
		Group		string			`json:"group"`
		Name		string			`json:"name"`
		Keywords	[]keywords.Keyword	`json:"keywords"`
	}

	Resource struct {
		ResourceId	string			`json:"rid"`
		Subset		string			`json:"subset"`
		Log		LogType			`json:"-"`
	}

	ResourceStatusType struct {
		Label		string			`json:"label"`
		Status		status.StatusType	`json:"status"`
		Subset		string			`json:"subset,omitempty"`
		Type		string			`json:"type"`
		Log		[]LogEntry		`json:"log,omitempty"`
	}
)


func (r Resource) String() string {
	return fmt.Sprintf("<Resource %s>", r.ResourceId)
}

func (r Resource) GetSubset() string {
	return r.Subset
}

func (r *Resource) GetLog() *LogType {
	return &r.Log
}

func ResourceType(r ResourceInterface) string {
	m := r.Manifest()
	return fmt.Sprintf("%s.%s", m.Group, m.Name)
}

func ResourceLabel(r ResourceInterface) string {
	return fmt.Sprintf("%s %s", ResourceType(r), r.Label())
}

func Start(r ResourceInterface) error {
	return r.Start()
}

func Stop(r ResourceInterface) error {
	return r.Stop()
}

func Status(r ResourceInterface) status.StatusType {
	return r.Status()
}

func PrintStatus(r ResourceInterface) error {
	data :=  ResourceStatusType {
		Label: ResourceLabel(r),
		Type: ResourceType(r),
		Status: Status(r),
		Subset: r.GetSubset(),
		Log: r.GetLog().Dump(),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func PrintManifest(r ResourceInterface) error {
	m := r.Manifest()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(m)
}

func PrintHelp(r ResourceInterface) error {
	fmt.Println(`Environment variables:
  RES_ACTION=start|stop|status|manifest

Stdin:
  json formatted context data
	`)
	return nil
}

func Action(r ResourceInterface) error {
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
	return nil
}

