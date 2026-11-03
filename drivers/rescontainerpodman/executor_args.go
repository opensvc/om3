package rescontainerpodman

import (
	"context"
	"os/exec"
	"strings"

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
	if !hasNetworkOption(a.Get()) {
		// A container with no netns keyword nor a network in its
		// run_args has a private netns, as the keyword says and as v2
		// and the docker driver do, rather than the default network of
		// podman: an om network ip in the netns would keep the default
		// route of that network, which its replies would take.
		a.Append("--net", "none")
	}
	return a, nil
}

// hasNetworkOption says the run args name the network of the container, as
// "--net x", "--network x" or "--net=x".
func hasNetworkOption(l []string) bool {
	for i, s := range l {
		switch {
		case s == "--net" || s == "--network":
			if i+1 < len(l) {
				return true
			}
		case strings.HasPrefix(s, "--net=") || strings.HasPrefix(s, "--network="):
			return true
		}
	}
	return false
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
