package ping

import (
	"context"
	"os/exec"
	"time"
)

func Ping(s string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return PingContext(ctx, s)
}

func PingContext(ctx context.Context, s string) (bool, error) {
	cmd := exec.CommandContext(ctx, "ping", "-i", "1", "-c", "1", s)
	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
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
