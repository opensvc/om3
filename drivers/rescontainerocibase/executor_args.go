package rescontainerocibase

import (
	"encoding/json"
	"fmt"

	"github.com/opensvc/om3/util/plog"
)

type (
	// ExecutorArg implements ExecutorArgser
	ExecutorArg struct {
		BT *BT
	}
)

func (ea *ExecutorArg) HasImageArgs() Args {
	return Args{
		{Option: "image"},
		{Option: "inspect", Value: ea.BT.Image, HasValue: true},
	}
}

func (ea *ExecutorArg) InspectArgs() Args {
	return Args{
		{Option: "container"},
		{Option: "inspect"},
		{Option: "--format", Value: "{{json .}}", HasValue: true},
		{Option: ea.BT.ContainerName()},
	}
}

func (ea *ExecutorArg) InspectParser(b []byte) (Inspecter, error) {
	data := &InspectData{}
	if err := json.Unmarshal(b, data); err != nil {
		return nil, err
	} else {
		return data, nil
	}
}

func (ea *ExecutorArg) Log() *plog.Logger {
	return ea.BT.Log()
}

func (ea *ExecutorArg) PullArgs() Args {
	return Args{
		{Option: "image"},
		{Option: "pull", Value: ea.BT.Image, HasValue: true},
	}
}

func (ea *ExecutorArg) RemoveArgs() Args {
	return Args{
		{Option: "container"},
		{Option: "rm", Value: ea.BT.ContainerName(), HasValue: true},
	}
}

func (ea *ExecutorArg) RunArgs() Args {
	bt := ea.BT
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

func (ea *ExecutorArg) StartArgs() Args {
	return Args{
		{Option: "container"},
		{Option: "start", Value: ea.BT.executer.Inspect().ID(), HasValue: true},
	}
}

func (ea *ExecutorArg) StopArgs() Args {
	a := Args{
		{Option: "container"},
		{Option: "stop", Value: ea.BT.ContainerName(), HasValue: true},
	}
	if ea.BT.StopTimeout != nil && *ea.BT.StopTimeout > 0 {
		arg := Arg{
			Option:   "--time",
			Value:    fmt.Sprintf("%.0f", ea.BT.StopTimeout.Seconds()),
			HasValue: true,
		}
		a = append(a, arg)
	}
	return a
}
