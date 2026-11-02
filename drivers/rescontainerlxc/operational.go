package rescontainerlxc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

const (
	// stopKillMargin is what lxc-stop is left, past the time it waits for
	// the init to shut the container down, to kill it within stop_timeout.
	stopKillMargin = 5

	// operationalRetry is how long the wait for the boot of a container
	// sleeps between two looks.
	operationalRetry = 200 * time.Millisecond
)

func (t *T) hasEncap() bool {
	return slices.Contains(t.EncapNodes, t.GetHostname())
}

// waitOperational waits for the container to have booted, for the encap
// resources to start in it: its init answers a command, and, when it is
// systemd, the system is running, or degraded, as it ends in a lxc container
// whose units fail for lack of what the host keeps for itself, as
// systemd-sysusers. A system in maintenance or stopping is an error. With
// another init, nothing tells the boot is done: the command answering is all
// that is waited for.
//
// The wait ends with the context, the start timeout of the resource.
func (t *T) waitOperational(ctx context.Context) error {
	if err := t.waitAttach(ctx); err != nil {
		return err
	}
	pid, err := t.getPID(ctx)
	if err != nil || pid == 0 {
		return err
	}
	if !isSystemd(pid) {
		t.Log().Infof("the init of container %s is no systemd: no boot to wait for, its encap resources start now", t.Name)
		return nil
	}
	return t.waitSystemRunning(ctx)
}

// isSystemd says the init of pid is systemd.
func isSystemd(pid int) bool {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	return err == nil && strings.TrimSpace(string(b)) == "systemd"
}

// attach runs args in the container, and returns what it wrote.
func (t *T) attach(ctx context.Context, args ...string) (string, error) {
	base, err := t.rcmd(ctx, nil)
	if err != nil {
		return "", err
	}
	if len(base) == 0 {
		return "", errors.New("no command to run in the container")
	}
	b, err := exec.CommandContext(ctx, base[0], append(base[1:], args...)...).CombinedOutput()
	return strings.TrimSpace(string(b)), err
}

// waitAttach waits for the init of the container to run a command.
func (t *T) waitAttach(ctx context.Context) error {
	for {
		_, err := t.attach(ctx, "true")
		if err == nil {
			return nil
		}
		if err := sleepCtx(ctx, operationalRetry); err != nil {
			return fmt.Errorf("container %s runs no command: %w", t.Name, err)
		}
	}
}

// waitSystemRunning waits for systemd to say the system is running, or
// degraded. systemctl --wait does not wait for a bus not up yet, and exits 0
// for running only: what it says is read, and asked again until it says one
// of the states a boot ends in.
func (t *T) waitSystemRunning(ctx context.Context) error {
	var last string
	for {
		out, _ := t.attach(ctx, "systemctl", "is-system-running", "--wait")
		state := lastLine(out)
		switch state {
		case "running":
			t.Log().Infof("container %s booted: the system is running", t.Name)
			return nil
		case "degraded":
			failed, _ := t.attach(ctx, "systemctl", "--failed", "--no-legend", "--plain")
			t.Log().Infof("container %s booted: the system is degraded, failed units: %s", t.Name, strings.Join(strings.Fields(unitNames(failed)), " "))
			return nil
		case "maintenance", "stopping":
			return fmt.Errorf("container %s: the system is %s", t.Name, state)
		}
		last = state
		if err := sleepCtx(ctx, operationalRetry); err != nil {
			if last == "" {
				last = "not answering"
			}
			return fmt.Errorf("container %s did not boot, the system is %s: %w", t.Name, last, err)
		}
	}
}

// unitNames returns the first column of the lines of systemctl --failed.
func unitNames(s string) string {
	l := make([]string, 0)
	for _, line := range strings.Split(s, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			l = append(l, fields[0])
		}
	}
	return strings.Join(l, " ")
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
