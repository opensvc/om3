package rescontainerdocker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/opensvc/om3/util/args"
	"github.com/opensvc/om3/util/capabilities"
)

// RunArgsBase append extra args for docker
func (ea *ExecutorArg) RunArgsBase(ctx context.Context) (*args.T, error) {
	a, err := ea.ExecutorArg.RunArgsBase(ctx)
	if err != nil {
		return nil, err
	}
	if len(ea.BT.UserNS) > 0 {
		if ea.BT.UserNS != "host" {
			return nil, fmt.Errorf("unexpected userns value '%s': the only valid value is 'hosts'", ea.BT.UserNS)
		}
		a.Append("--userns", ea.BT.UserNS)
	}

	if !a.HasOptionAndAnyValue("--net") {
		// Use the --net none Docker option when nets is unset and --net is not present in run_args
		a.Append("--net", "none")
	}

	return a, nil
}

func (ea *ExecutorArg) WaitNotRunning(ctx context.Context) error {
	var cmd *exec.Cmd
	a := []string{"container", "wait", ea.BT.ContainerName()}
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
	ea.Log().Infof("%s %s", ea.exe, strings.Join(a, " "))
	if err := cmd.Run(); err != nil {
		ea.Log().Infof("%s %s: %s", ea.exe, strings.Join(a, " "), err)
		return err
	}
	return nil
}

func (ea *ExecutorArg) WaitRemoved(ctx context.Context) error {
	if ctx != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	if removed, err := ea.isRemoved(ctx); err != nil {
		return err
	} else if removed {
		return nil
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if removed, err := ea.isRemoved(ctx); err != nil {
				return err
			} else if removed {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (ea *ExecutorArg) isRemoved(ctx context.Context) (bool, error) {
	if inspect, err := ea.inspectRefresher.InspectRefresh(ctx); err == nil {
		ea.BT.Log().Tracef("is removed: %v", inspect == nil)
		return inspect == nil, nil
	} else {
		ea.BT.Log().Tracef("is removed: false")
		return false, err
	}
}

func timeoutFlag() string {
	if capabilities.Has(capHasTimeoutFlag) {
		return "--timeout"
	} else {
		return "--time"
	}
}

func (ea *ExecutorArg) StopArgs() *args.T {
	a := args.New("container", "stop", ea.BT.ContainerName())
	if ea.BT.StopTimeout != nil && *ea.BT.StopTimeout > 0 {
		a.Append(timeoutFlag(), fmt.Sprintf("%.0f", ea.BT.StopTimeout.Seconds()))
	}
	return a
}
