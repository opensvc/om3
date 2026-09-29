//go:build linux

package resiprule

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actionresdeps"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/command"
)

func (t *T) ActionResourceDeps() []actionresdeps.Dep {
	return []actionresdeps.Dep{
		{Action: "start", A: t.RID(), B: t.NetNS},
		{Action: "start", A: t.NetNS, B: t.RID()},
		{Action: "stop", A: t.NetNS, B: t.RID()},
	}
}

func (t *T) LinkTo() string {
	return t.NetNS
}

// Start adds the rule in the network namespace of the container, unless it
// is there already: "ip rule add" is not idempotent, and would add it twice.
func (t *T) Start(ctx context.Context) error {
	path, err := t.netNSPath(ctx)
	if err != nil {
		return err
	}
	if up, err := t.isUp(path); err != nil {
		return err
	} else if up {
		t.Log().Infof("rule is already up")
		return nil
	}
	return t.run(path, "add")
}

// Stop deletes the rule from the network namespace of the container. A
// container gone with its namespace took the rule with it.
func (t *T) Stop(ctx context.Context) error {
	path, err := t.netNSPath(ctx)
	if err != nil {
		t.Log().Infof("skip: %s", err)
		return nil
	}
	if up, err := t.isUp(path); err != nil {
		return err
	} else if !up {
		t.Log().Infof("rule is already down")
		return nil
	}
	return t.run(path, "del")
}

// Status is up when the rule is listed in the network namespace of the
// container, and down when the namespace is not there.
func (t *T) Status(ctx context.Context) status.T {
	path, err := t.netNSPath(ctx)
	if err != nil {
		return status.Down
	}
	up, err := t.isUp(path)
	if err != nil {
		t.StatusLog().Warn("%s", err)
		return status.Undef
	}
	if up {
		return status.Up
	}
	return status.Down
}

func (t *T) netNSPath(ctx context.Context) (string, error) {
	r := t.GetObjectDriver().ResourceByID(t.NetNS)
	if r == nil {
		return "", fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	}
	i, ok := r.(resource.NetNSPather)
	if !ok {
		return "", fmt.Errorf("resource %s pointed by the netns keyword does not expose a netns path", t.NetNS)
	}
	path, err := i.NetNSPath(ctx)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("resource %s has no network namespace", t.NetNS)
	}
	return path, nil
}

// args returns the "ip rule <verb> <spec>" command run in the namespace at
// path. A rule selecting an IPv6 address goes to the IPv6 rule table, which
// "ip rule list" does not show without -6.
func (t *T) args(path, verb string) []string {
	args := []string{"--net=" + path, "ip"}
	if isIPv6Spec(t.Spec) {
		args = append(args, "-6")
	}
	args = append(args, "rule", verb)
	return append(args, t.Spec...)
}

func (t *T) run(path, verb string) error {
	return command.New(
		command.WithName("nsenter"),
		command.WithArgs(t.args(path, verb)),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) isUp(path string) (bool, error) {
	cmd := command.New(
		command.WithName("nsenter"),
		command.WithArgs(t.args(path, "list")),
		command.WithLogger(t.Log()),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
	)
	if err := cmd.Run(); err != nil {
		if b := strings.TrimSpace(string(cmd.Stderr())); b != "" {
			return false, fmt.Errorf("%s: %s", err, b)
		}
		return false, err
	}
	return strings.TrimSpace(string(cmd.Stdout())) != "", nil
}
