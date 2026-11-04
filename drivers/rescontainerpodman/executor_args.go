package rescontainerpodman

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/opensvc/om3/v3/drivers/rescontainerocibase"
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
	if !rescontainerocibase.HasNetworkOption(a.Get()) {
		// A container with no netns keyword nor a network in its
		// run_args has a private netns, as the keyword says and as the
		// docker driver does, rather than the default network of
		// podman, which v2 used: an om network ip in the netns would
		// keep the default route of that network, which its replies
		// would take.
		a.Append("--net", "none")
	}
	if unprivilegedPortsSysctlWanted(a.Get()) {
		// As docker does: a process of a container with its own netns
		// listens on any port, as a non-root haproxy on 80, where podman
		// leaves the kernel default, ports under 1024 for root only. The
		// netns is the container's own: the node ports are not opened.
		a.Append("--sysctl", unprivilegedPortsSysctl+"=0")
	}
	return a, nil
}

// unprivilegedPortsSysctl is the sysctl setting the lowest port a process
// not root may listen on, 1024 by default.
const unprivilegedPortsSysctl = "net.ipv4.ip_unprivileged_port_start"

// unprivilegedPortsSysctlFile is the file of unprivilegedPortsSysctl, absent
// from the kernels older than 4.11, which have no such sysctl.
var unprivilegedPortsSysctlFile = "/proc/sys/net/ipv4/ip_unprivileged_port_start"

// unprivilegedPortsSysctlWanted says the container of the run args l has its
// own netns, and no setting of unprivilegedPortsSysctl: a netns of the node
// or of another container is not the container's to set, as docker does
// not, and the setting the user gives wins.
func unprivilegedPortsSysctlWanted(l []string) bool {
	if _, err := os.Stat(unprivilegedPortsSysctlFile); err != nil {
		return false
	}
	network := ""
	for i, s := range l {
		switch {
		case (s == "--net" || s == "--network") && i+1 < len(l):
			network = l[i+1]
		case strings.HasPrefix(s, "--net="):
			network = strings.TrimPrefix(s, "--net=")
		case strings.HasPrefix(s, "--network="):
			network = strings.TrimPrefix(s, "--network=")
		case s == "--sysctl" && i+1 < len(l) && strings.HasPrefix(l[i+1], unprivilegedPortsSysctl+"="):
			return false
		case strings.HasPrefix(s, "--sysctl="+unprivilegedPortsSysctl+"="):
			return false
		}
	}
	switch {
	case network == "host":
		return false
	case strings.HasPrefix(network, "container:"), strings.HasPrefix(network, "ns:"):
		return false
	}
	return true
}

// WaitRemoved waits for the container to be removed. podman 1 has no
// --condition: the wait fails there, and the executor inspects the container
// until it is gone.
func (ea *ExecutorArg) WaitRemoved(ctx context.Context) error {
	return ea.wait(ctx, "container", "wait", "--condition", "removing", ea.BT.ContainerName())
}

// WaitNotRunning waits for the container to be not running: a wait with no
// --condition waits for it to be stopped, which every podman version does,
// podman 1 knowing no --condition and podman before 4.3 no --ignore. A
// container gone fails the wait, which the executor judges by inspect.
func (ea *ExecutorArg) WaitNotRunning(ctx context.Context) error {
	return ea.wait(ctx, "container", "wait", ea.BT.ContainerName())
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
	cred, err := ea.Credential()
	if err != nil {
		return err
	}
	cred.Demote(cmd)
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
