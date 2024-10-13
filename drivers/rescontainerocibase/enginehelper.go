package rescontainerocibase

import (
	"encoding/json"

	"github.com/opensvc/om3/util/plog"
)

type (
	EngineHelper interface {
		StartArgs() Args
		StopArgs() Args
		PullArgs() Args
		RunArgs() Args
		RemoveArgs() Args
		WaitArgs(...WaitCondition) Args
		HasImageArgs() Args
		InspectArgs() Args
		InspectParser([]byte) (Inspecter, error)
		IsNotFound(err error) bool
		Log() *plog.Logger
	}

	EngineHelp struct {
		BT *BT
	}
)

func (eh *EngineHelp) Log() *plog.Logger {
	return eh.BT.Log()
}

func (eh *EngineHelp) StartArgs() Args {
	return Args{
		{Option: "container"},
		{Option: "start", Value: eh.BT.c.Inspect().ID(), HasValue: true},
	}
}

func (eh *EngineHelp) StopArgs() Args {
	return Args{
		{Option: "container"},
		{Option: "kill", Value: eh.BT.ContainerName(), HasValue: true},
	}
}

func (eh *EngineHelp) PullArgs() Args {
	return Args{
		{Option: "image"},
		{Option: "pull", Value: eh.BT.Image, HasValue: true},
	}
}

func (eh *EngineHelp) RunArgs() Args {
	bt := eh.BT
	a := Args{
		{Option: "container"},
		{Option: "run"},
		{Option: "--name", Value: bt.ContainerName(), HasValue: true},
	}
	for k, v := range bt.Labels() {
		arg := Arg{
			Option:   "--label",
			Multi:    true,
			Value:    k + "=" + v,
			HasValue: true,
		}
		a = append(a, arg)
	}
	if bt.Detach {
		a = append(a, Arg{Option: "--detach"})
	}
	if s, err := bt.FormatNS(bt.NetNS); err != nil {
		// TODO add error
		panic("FormatNS")
	} else if s == "" {
		a = append(a, Arg{Option: "--net", Value: "none", HasValue: true})
	} else {
		a = append(a, Arg{Option: "--net", Value: s, HasValue: true})
	}
	// TODO fix this
	a = append(a, Arg{Option: bt.Image})
	return a
}

func (eh *EngineHelp) RemoveArgs() Args {
	return Args{
		{Option: "container"},
		{Option: "rm", Value: eh.BT.ContainerName(), HasValue: true},
	}
}

func (eh *EngineHelp) HasImageArgs() Args {
	return Args{
		{Option: "image"},
		{Option: "inspect", Value: eh.BT.Image, HasValue: true},
	}
}

func (eh *EngineHelp) InspectArgs() Args {
	return Args{
		{Option: "container"},
		{Option: "inspect"},
		{Option: "--format", Value: "{{json .}}", HasValue: true},
		{Option: eh.BT.ContainerName()},
	}
}

func (eh *EngineHelp) InspectParser(b []byte) (Inspecter, error) {
	data := &InspectData{}
	if err := json.Unmarshal(b, data); err != nil {
		return nil, err
	} else {
		return data, nil
	}
}
