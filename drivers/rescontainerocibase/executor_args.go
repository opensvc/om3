package rescontainerocibase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

func (ea *ExecutorArg) HasImageArgs() Args {
	return Args{
		{Option: "image"},
		{Option: "inspect"},
		{Option: "--format", Value: "{{.ID}}", HasValue: true},
		{Option: ea.BT.Image},
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

func (ea *ExecutorArg) RunArgs() (Args, error) {
	bt := ea.BT
	ea.runArgsEnvM = make(map[string]string)
	a := Args{
		{Option: "container"},
		{Option: "run"},
		{Option: "--name", Value: bt.ContainerName(), HasValue: true},
	}
	if bt.Hostname != "" {
		a = append(a, Arg{Option: "--hostname", Value: bt.Hostname, HasValue: true})
	}
	if bt.TTY {
		a = append(a, Arg{Option: "--tty"})
	}
	if bt.Detach {
		a = append(a, Arg{Option: "--detach"})
	}
	if bt.Privileged {
		a = append(a, Arg{Option: "--privileged"})
	}
	if bt.Interactive {
		a = append(a, Arg{Option: "--interactive"})
	}
	if len(bt.Entrypoint) > 0 {
		a = append(a, Arg{Option: "--entrypoint", Value: bt.Entrypoint[0], HasValue: true})
	}

	for _, f := range []func() (Args, error){
		ea.runArgsForNS,
		ea.runArgsMounts,
		ea.runArgsEnv,
	} {
		if args, err := f(); err != nil {
			return a, err
		} else {
			a = append(a, args...)
		}
	}
	a = append(a, ea.runArgsLabels()...)
	a = append(a, ea.runArgsDNS()...)
	a = append(a, ea.runArgsDNSSearch()...)
	a = append(a, ea.runArgsDNSOption()...)
	a = append(a, ea.runArgsCGroupParent()...)

	for _, v := range bt.Devices {
		a = append(a, Arg{Option: "--device", Value: v, Multi: true, HasValue: true})
	}

	if ea.BT.Remove {
		a = append(a, Arg{Option: "--rm"})
	}

	// TODO: merge run_args
	for _, v := range bt.RunArgs {
		a = append(a, Arg{Option: v})
	}

	a = append(a, Arg{Option: bt.Image})

	a = append(a, ea.runArgsCommand()...)

	return a, nil
}

func (ea *ExecutorArg) RunCmdEnv() (map[string]string, error) {
	return ea.runArgsEnvM, nil
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
		{Option: "stop"},
		{Option: ea.BT.ContainerName()},
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

func (ea *ExecutorArg) WaitNotRunning(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (ea *ExecutorArg) WaitRemoved(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (ea *ExecutorArg) runArgsCGroupParent() Args {
	if !ea.RunArgsCGroupParentDisable || ea.BT.PG.ID == "" {
		return nil
	}
	return Args{
		{Option: "--cgroup-parent", Value: ea.BT.PG.ID, HasValue: true},
	}
}

func (ea *ExecutorArg) runArgsCommand() Args {
	a := make(Args, 0, len(ea.BT.Command))
	for _, s := range ea.BT.Command {
		a = append(a, Arg{Option: s})
	}
	return a
}

func (ea *ExecutorArg) runArgsDNS() Args {
	if !ea.needDNS() {
		return nil
	}
	return multiArgs("--dns", ea.BT.DNS...)
}

func (ea *ExecutorArg) runArgsDNSOption() Args {
	if !ea.needDNS() {
		return nil
	}
	return multiArgs(ea.RunArgsDNSOptionOption, "ndots:2", "edns0", "use-vc")
}

func (ea *ExecutorArg) runArgsDNSSearch() Args {
	if len(ea.BT.DNSSearch) > 0 {
		return multiArgs("--dns-search", ea.BT.DNSSearch...)
	}
	if !ea.needDNS() {
		return nil
	}
	dom0 := ea.BT.ObjectDomain
	dom1 := strings.SplitN(dom0, ".", 2)[1]
	dom2 := strings.SplitN(dom1, ".", 2)[1]
	return multiArgs("--dns-search", dom0, dom1, dom2)
}

func (ea *ExecutorArg) runArgsEnv() (Args, error) {
	if l, m, err := ea.BT.GenEnv(); err != nil {
		return nil, err
	} else {
		ea.runArgsEnvM = m
		args := make(Args, 0, len(l))
		for _, v := range l {
			args = append(args, Arg{Option: "-e", Value: v, HasValue: true, Multi: true})
		}
		return args, nil
	}
}

func (ea *ExecutorArg) runArgsForNS() (Args, error) {
	type nsCandidate struct {
		kw  string
		opt string
		ns  string
	}
	bt := ea.BT
	nsCandidates := []nsCandidate{
		{kw: "netns", opt: "--net", ns: bt.NetNS},
		{kw: "pidns", opt: "--pid", ns: bt.PIDNS},
		{kw: "ipcns", opt: "--ipc", ns: bt.IPCNS},
		{kw: "utsns", opt: "--uts", ns: bt.UTSNS},
		{kw: "userns", opt: "--userns", ns: bt.UserNS},
	}
	var a Args
	for _, c := range nsCandidates {
		if value, err := ea.BT.FormatNS(c.ns); err != nil {
			return a, fmt.Errorf("unable to prepare option '%s' from kw setting '%s=%s': %s", c.opt, c.kw, c.ns, err)
		} else if value != "" {
			a = append(a, Arg{Option: c.opt, Value: value, HasValue: true})
		}
	}
	return a, nil
}

func (ea *ExecutorArg) runArgsLabels() []Arg {
	m := ea.BT.Labels()
	labels := make([]string, 0, len(m))
	for k, v := range m {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	return multiArgs("--label", labels...)
}

func (ea *ExecutorArg) runArgsMounts() (Args, error) {
	args := make(Args, 0)
	mounts, err := ea.BT.Mounts()
	if err != nil {
		return args, err
	}
	for _, m := range mounts {
		if !file.Exists(m.Source) {
			ea.Log().Infof("create missing mount source %s", m.Source)
			if err := os.MkdirAll(m.Source, os.ModePerm); err != nil {
				return nil, err
			}
		}
		args = append(args, Arg{
			Option:   "--volume",
			Value:    fmt.Sprintf("%s:%s:%s", m.Source, m.Target, m.Option),
			Multi:    true,
			HasValue: true,
		})
	}
	return args, nil
}

func (ea *ExecutorArg) needDNS() bool {
	switch ea.BT.NetNS {
	case "", "none":
		return true
	default:
		return false
	}
}

func multiArgs(option string, l ...string) Args {
	a := make(Args, 0, len(l))
	for _, s := range l {
		a = append(a, Arg{Option: option, Value: s, HasValue: true, Multi: true})
	}
	return a
}
