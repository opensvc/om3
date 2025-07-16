package ping

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

type (
	T struct {
		Dev      string
		Ctx      context.Context
		Dst      string
		V        int
		Interval int
		Count    int
	}
)

func Ping(s string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return PingContext(ctx, s)
}

func PingContext(ctx context.Context, s string) (bool, error) {
	t := T{
		Ctx:      ctx,
		Dst:      s,
		Count:    1,
		Interval: 1,
	}
	return t.Ping()
}

func (t *T) Ping() (bool, error) {
	var (
		name string
		args []string
	)
	if t.V == 6 {
		name = "ping6"
	} else {
		name = "ping"
	}
	if t.Count > 0 {
		args = append(args, "-c", fmt.Sprint(t.Count))
	}
	if t.Interval > 0 {
		args = append(args, "-i", fmt.Sprint(t.Interval))
	}
	if t.Dev != "" {
		args = append(args, "-I", t.Dev)
	}
	args = append(args, t.Dst)

	var cmd *exec.Cmd
	if t.Ctx != nil {
		cmd = exec.CommandContext(t.Ctx, name, args...)
	} else {
		cmd = exec.Command(name, args...)
	}
	err := cmd.Run()
	if err != nil {
		if t.Ctx.Err() == context.DeadlineExceeded {
			return false, nil
		}
		if cmd.ProcessState.ExitCode() == 1 {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}
