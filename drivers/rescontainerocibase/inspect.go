package rescontainerocibase

import (
	"encoding/json"
	"strings"
)

type (
	// InspectData implements Inspecter
	InspectData struct {
		Id                    string
		Image                 string
		InspectDataConfig     InspectDataConfig     `json:"Config"`
		InspectDataHostConfig InspectDataHostConfig `json:"HostConfig"`
		NetworkSettings       struct {
			SandboxKey string
		}
		State InspectDataState
	}

	InspectDataConfig struct {
		Entrypoint InspectDataConfigEntrypoint
		Hostname   string
		OpenStdin  bool
		Tty        bool
	}

	InspectDataConfigEntrypoint []string

	InspectDataHostConfig struct {
		AutoRemove  bool
		IpcMode     string
		Privileged  bool
		NetworkMode string
		PidMode     string
		UsernsMode  string
		UTSMode     string
	}

	InspectDataState struct {
		Pid     int
		Running bool
		Status  string
	}
)

func (i *InspectData) Config() *InspectDataConfig {
	if i == nil {
		return nil
	}
	return &i.InspectDataConfig
}

func (i *InspectData) Defined() bool {
	if i != nil {
		return true
	}
	return false
}

func (i *InspectData) HostConfig() *InspectDataHostConfig {
	if i == nil {
		return nil
	}
	return &i.InspectDataHostConfig
}

func (i *InspectData) ID() string {
	if i == nil {
		return ""
	}
	return i.Id
}

func (i *InspectData) ImageID() string {
	if i == nil {
		return ""
	}
	return i.Image
}

func (i *InspectData) PID() int {
	if i == nil {
		return 0
	}
	return i.State.Pid
}

func (i *InspectData) Running() bool {
	if i == nil {
		return false
	}
	return i.State.Running
}

func (i *InspectData) Status() string {
	if i == nil {
		return ""
	}
	return i.State.Status
}

func (i *InspectData) SandboxKey() string {
	if i == nil {
		return ""
	}
	return i.NetworkSettings.SandboxKey
}

func (i *InspectDataConfigEntrypoint) UnmarshalJSON(b []byte) error {
	var j any
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	switch v := j.(type) {
	case string:
		*i = strings.Split(v, " ")
	case []string:
		*i = v
	}
	return nil
}
