package evwatch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ProbeHelp checks a tool supports an event stream, by the help it prints:
// name with args, the variables of env added, prints flag.
//
// Only a tool that ran to its end and printed no flag is unsupported, the
// error wrapping ErrUnsupported: an older tool, whose help has no such
// option, or no such command and exits in error. A probe that timed out, or
// could not run, says nothing of the tool, and its error is retried, as an
// engine busy or a node loaded.
func ProbeHelp(ctx context.Context, env []string, flag, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.WaitDelay = streamWaitDelay
	b, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), ctx.Err())
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	if !strings.Contains(string(b), flag) {
		return fmt.Errorf("%s %s prints no %s: %w", name, strings.Join(args, " "), flag, ErrUnsupported)
	}
	return nil
}
