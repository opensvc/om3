package rescontainerpodman

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/drivers/rescontainer"
	"github.com/opensvc/om3/v3/util/args"
)

// RunArgsBase append extra args for podman
func (ea *ExecutorArg) RunArgsBase(ctx context.Context) (*args.T, error) {
	a := args.New()
	// TODO: "cni-config-dir", ..., for other Args ?
	if base, err := ea.ExecutorArg.RunArgsBase(ctx); err != nil {
		return nil, err
	} else {
		a.Append(base.Get()...)
	}
	if len(ea.BT.UserNS) > 0 {
		isRawValue := func(s string) bool {
			return strings.HasPrefix(s, "auto") ||
				s == "host" ||
				strings.HasPrefix(s, "keep-id") ||
				strings.HasPrefix(s, "nomap") ||
				strings.HasPrefix(s, "ns:")
		}

		if isRawValue(ea.BT.UserNS) {
			a.Append("--userns", ea.BT.UserNS)
		} else if s, err := ea.BT.FormatNS(ea.BT.UserNS); err != nil {
			return nil, err
		} else {
			a.Append("--userns", s)
		}
	}
	if a.HasOptionAndMatchingValue("--net", "(^none$|^container:.*$)") ||
		a.HasOptionAndMatchingValue("--network", "(^none$|^container:.*$)") {
		// podman refuses the dns options in these two network modes:
		//
		//   conflicting options: dns and the network mode: none
		//   conflicting options: dns and the network mode: container
		//
		// and writes no /etc/resolv.conf of its own for them either, so
		// dropping the options left the container of an object using the
		// pause container model with no resolver at all: it could not resolve
		// a name of its own cluster. Hand it the file the options would have
		// produced instead.
		a.DropOptionAndAnyValue("--dns")
		a.DropOptionAndAnyValue("--dns-opt")
		a.DropOptionAndAnyValue("--dns-option")
		a.DropOptionAndAnyValue("--dns-search")
		if mount, err := ea.resolvConfMount(); err != nil {
			return nil, err
		} else if mount != "" {
			a.DropOptionAndExactValue("-v", mount)
			a.DropOptionAndExactValue("--volume", mount)
			a.Append("-v", mount)
		}
	}
	return a, nil
}

// resolvConfMount writes the resolver configuration of the container and
// returns the option mounting it, or an empty string when there is nothing to
// say to the container.
//
// The file is written on every start, and a container reads it for as long as
// it runs: a container adapts to a cluster layout change by being restarted.
func (ea *ExecutorArg) resolvConfMount() (string, error) {
	resolvConf := rescontainer.ResolvConf{
		Nameservers: ea.BT.DNS,
		Searches:    rescontainer.SearchDomains(ea.BT.ObjectDomain, ea.BT.DNSSearch),
		Options:     rescontainer.ResolvConfOptions,
	}
	if resolvConf.IsZero() {
		return "", nil
	}
	if n := len(resolvConf.Nameservers); n > rescontainer.MaxNameservers {
		ea.BT.Log().Warnf("cluster.dns names %d nameservers, a resolver reads the first %d: %s is not written to the container resolv.conf",
			n, rescontainer.MaxNameservers, strings.Join(resolvConf.Nameservers[rescontainer.MaxNameservers:], ", "))
	}
	path, err := rescontainer.WriteResolvConf(filepath.Join(ea.BT.VarDir(), "resolv.conf"), resolvConf)
	if err != nil {
		return "", err
	}
	return path + ":/etc/resolv.conf:ro", nil
}

func (ea *ExecutorArg) WaitRemoved(ctx context.Context) error {
	return ea.wait(ctx, "container", "wait", "--ignore", "--condition", "removing", ea.BT.ContainerName())
}

func (ea *ExecutorArg) WaitNotRunning(ctx context.Context) error {
	return ea.wait(ctx, "container", "wait", "--ignore", "--condition", "stopped", ea.BT.ContainerName())
}

func (ea *ExecutorArg) wait(ctx context.Context, a ...string) error {
	var cmd *exec.Cmd

	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cmd = exec.CommandContext(ctx, ea.exe, a...)
		}
	} else {
		cmd = exec.Command(ea.exe, a...)
	}
	ea.BT.Log().Infof("%s %s", ea.exe, strings.Join(a, " "))
	if err := cmd.Run(); err != nil {
		ea.BT.Log().Tracef("%s %s: %s", ea.exe, strings.Join(a, " "), err)
		return err
	}
	return nil
}

func (ea *ExecutorArg) ExecBaseArgs() []string {
	return ea.baseArgs
}
