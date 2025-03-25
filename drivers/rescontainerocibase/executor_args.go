package rescontainerocibase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/opensvc/om3/util/args"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/plog"
)

type (
	// ExecutorArg implements ExecutorArgser
	ExecutorArg struct {
		BT *BT

		// RunArgsCGroupParentDisable disable the "--cgroup-parent" RunArgs setting
		RunArgsCGroupParentDisable bool

		// RunArgsDNSOptionOption is the option name used during RunArgs
		// to set container dns options (example "--dns-option").
		RunArgsDNSOptionOption string

		// runArgsEnvM is internal store for the environment variables that
		// must be added to the exec.Cmd Env, during the Executor.Run() call.
		// It is returned by ExecutorArg.RunCmdEnv() calls.
		// Its value is computed during ExecutorArg.RunArgs() from the BT.GenEnv()
		// results.
		runArgsEnvM map[string]string
	}
)

func (ea *ExecutorArg) EnterCmdArgs() []string {
	return []string{"exec", "-it", ea.BT.ContainerName()}
}

func (ea *ExecutorArg) EnterCmdCheckArgs() []string {
	return []string{"exec", ea.BT.ContainerName()}
}

func (ea *ExecutorArg) HasImageArgs() *args.T {
	return args.New("image", "inspect", "--format", "{{.ID}}", ea.BT.Image)
}

func (ea *ExecutorArg) InspectArgs() *args.T {
	return args.New("container", "inspect", "--format", "{{json .}}", ea.BT.ContainerName())
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

func (ea *ExecutorArg) PullArgs() *args.T {
	return args.New("image", "pull", ea.BT.Image)
}

func (ea *ExecutorArg) RemoveArgs() *args.T {
	return args.New("container", "rm", ea.BT.ContainerName())
}

func (ea *ExecutorArg) RunArgsBase(ctx context.Context) (*args.T, error) {
	bt := ea.BT
	runArgs := args.New(bt.RunArgs...)

	ea.runArgsEnvM = make(map[string]string)

	runArgs.DropOptionAndAnyValue("--name")
	runArgs.DropOptionAndAnyValue("-n")
	a := args.New("container", "run", "--name", bt.ContainerName())

	if bt.Hostname != "" {
		runArgs.DropOptionAndAnyValue("-h")
		runArgs.DropOptionAndAnyValue("--hostname")
		a.Append("--hostname", bt.Hostname)
		// TODO: confirm ignore b2.1: if bt.NetNS != "host" && !strings.HasPrefix(bt.NetNS, "container#")
	}

	if bt.TTY {
		runArgs.DropOption("--tty")
		runArgs.DropOption("--t")
		a.Append("--tty")
	}

	if bt.Detach {
		runArgs.DropOption("--detach")
		runArgs.DropOption("-d")
		a.Append("--detach")
	}

	if bt.Privileged {
		runArgs.DropOption("--privileged")
		a.Append("--privileged")
	}

	if bt.ReadOnly != "" {
		runArgs.DropOption("--read-only")
		if bt.ReadOnly == "true" {
			a.Append("--read-only")
		}
	}

	if len(bt.User) > 0 {
		runArgs.DropOptionAndAnyValue("--user")
		runArgs.DropOptionAndAnyValue("-u")
		a.Append("--user", bt.User)
	}

	if bt.Interactive {
		runArgs.DropOption("--interactive")
		runArgs.DropOption("-i")
		a.Append("--interactive")
	}

	if len(bt.Entrypoint) > 0 {
		runArgs.DropOptionAndAnyValue("--entrypoint")
		a.Append("--entrypoint", bt.Entrypoint[0])
	}

	for _, ns := range []struct {
		optionName  string
		kwName      string
		kwValue     string
		dropOptions []string
	}{
		{optionName: "--net", kwName: "netns", kwValue: bt.NetNS, dropOptions: []string{"--net", "--network"}},
		{optionName: "--pid", kwName: "pidns", kwValue: bt.PIDNS, dropOptions: []string{"--pid"}},
		{optionName: "--ipc", kwName: "ipcns", kwValue: bt.IPCNS, dropOptions: []string{"--ipc"}},
		{optionName: "--uts", kwName: "utsns", kwValue: bt.UTSNS, dropOptions: []string{"--uts"}},
	} {
		if s, err := ea.BT.FormatNS(ns.kwValue); err != nil {
			return nil, fmt.Errorf("unable to prepare option '%s' from kw setting '%s=%s': %s", ns.optionName, ns.kwName, ns.kwValue, err)
		} else if s != "" {
			for _, dropOption := range ns.dropOptions {
				runArgs.DropOptionAndAnyValue(dropOption)
			}
			a.Append(ns.optionName, s)
		}
	}

	a.Append(ea.runArgsDNS()...)
	a.Append(ea.runArgsDNSSearch()...)
	a.Append(ea.runArgsDNSOption()...)
	a.Append(ea.runArgsCGroupParent()...)

	for _, v := range bt.Devices {
		runArgs.DropOptionAndExactValue("--device", v)
		a.Append("--device", v)
	}

	if mounts, err := ea.runArgsMounts(); err != nil {
		return a, err
	} else {
		for _, v := range mounts {
			runArgs.DropOptionAndExactValue("-v", v)
			runArgs.DropOptionAndExactValue("--volume", v)
			a.Append("--volume", v)
		}
	}
	if mounts, err := ea.runArgsEnv(ctx); err != nil {
		return a, err
	} else {
		for _, v := range mounts {
			runArgs.DropOptionAndExactValue("-e", v)
			a.Append("-e", v)
		}
	}

	a.Append(ea.runArgsLabels()...)

	a.Append(runArgs.Get()...)

	return a, nil
}

func (ea *ExecutorArg) RunArgsCommand() (*args.T, error) {
	return args.New(ea.BT.Command...), nil
}

func (ea *ExecutorArg) RunArgsImage() (*args.T, error) {
	return args.New(ea.BT.Image), nil
}

func (ea *ExecutorArg) RunCmdEnv() (map[string]string, error) {
	return ea.runArgsEnvM, nil
}

func (ea *ExecutorArg) StartArgs(ctx context.Context) (*args.T, error) {
	var id string
	if ea.BT == nil {
		return nil, fmt.Errorf("can't get start args from nil base container")
	} else if inspect, err := ea.BT.executer.Inspect(ctx); err != nil {
		return nil, fmt.Errorf("can't get start args: inspect: %w", err)
	} else if inspect == nil {
		return nil, fmt.Errorf("can't get start args from nil base container")
	} else if id = inspect.ID(); len(id) == 0 {
		return nil, fmt.Errorf("can't get start args from nil base container")
	}

	return args.New("container", "start", id), nil
}

func (ea *ExecutorArg) StopArgs() *args.T {
	a := args.New("container", "stop", ea.BT.ContainerName())
	if ea.BT.StopTimeout != nil && *ea.BT.StopTimeout > 0 {
		a.Append("--time", fmt.Sprintf("%.0f", ea.BT.StopTimeout.Seconds()))
	}
	return a
}

func (ea *ExecutorArg) WaitNotRunning(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (ea *ExecutorArg) WaitRemoved(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (ea *ExecutorArg) runArgsCGroupParent() []string {
	if !ea.RunArgsCGroupParentDisable || ea.BT.PG.ID == "" {
		return nil
	}
	return []string{"--cgroup-parent", ea.BT.PG.ID}
}

func (ea *ExecutorArg) runArgsDNS() []string {
	if !ea.needDNS() {
		return nil
	}
	a := make([]string, 0, 2*len(ea.BT.DNS))
	for _, s := range ea.BT.DNS {
		a = append(a, "--dns", s)
	}
	return a
}

func (ea *ExecutorArg) runArgsDNSOption() []string {
	if !ea.needDNS() {
		return nil
	}
	option := ea.RunArgsDNSOptionOption
	return []string{option, "ndots:2", option, "edns0", option, "use-vc"}
}

func (ea *ExecutorArg) runArgsDNSSearch() []string {
	if !ea.needDNS() {
		return nil
	}
	var a []string
	for _, s := range ea.BT.DNSSearch {
		a = append(a, "--dns-search", s)
	}

	dom0 := ea.BT.ObjectDomain
	if len(dom0) > 0 {
		a = append(a, "--dns-search", dom0)
		dom0S := strings.SplitN(dom0, ".", 2)
		if len(dom0S) > 1 {
			dom1 := dom0S[1]
			if len(dom1) > 0 {
				a = append(a, "--dns-search", dom1)
				dom1S := strings.SplitN(dom1, ".", 2)
				if len(dom1S) > 1 {
					dom2 := dom1S[1]
					a = append(a, "--dns-search", dom2)
				}
			}
		}
	}
	return a
}

func (ea *ExecutorArg) runArgsEnv(ctx context.Context) ([]string, error) {
	if l, m, err := ea.BT.GenEnv(ctx); err != nil {
		return nil, err
	} else {
		ea.runArgsEnvM = m
		return l, err
	}
}

func (ea *ExecutorArg) runArgsLabels() []string {
	m := ea.BT.Labels()
	a := make([]string, 0, 2*len(m))
	for k, v := range m {
		a = append(a, "--label", fmt.Sprintf("%s=%s", k, v))
	}
	return a
}

func (ea *ExecutorArg) runArgsMounts() ([]string, error) {
	mounts, err := ea.BT.Mounts()
	if err != nil {
		return nil, err
	}
	TargetToSource := make(map[string]string)
	a := make([]string, 0, len(mounts))
	for _, m := range mounts {
		if source, ok := TargetToSource[m.Target]; ok {
			return nil, fmt.Errorf("found at least two different volume mounts sources %s and %s that use the same destination %s",
				source, m.Source, m.Target)
		}
		TargetToSource[m.Target] = m.Source
		if !file.Exists(m.Source) {
			ea.Log().Infof("create missing mount source %s", m.Source)
			if err := os.MkdirAll(m.Source, os.ModePerm); err != nil {
				return nil, err
			}
		}
		// TODO: add b2.1 rule option: ro or rw
		a = append(a, fmt.Sprintf("%s:%s:%s", m.Source, m.Target, m.Option))
	}
	return a, nil
}

func (ea *ExecutorArg) needDNS() bool {
	switch ea.BT.NetNS {
	case "", "none":
		return true
	default:
		return false
	}
}
