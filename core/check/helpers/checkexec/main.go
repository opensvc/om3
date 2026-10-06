// Package checkexec runs the commands the check drivers read their values
// from.
//
// A node lacking the command of a check driver has nothing to report for it:
// the driver returns no result rather than an error, as the agent runs every
// driver on every node.
package checkexec

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Timeout bounds a command of a check driver: a check run reports what it
// can, and a hung tool does not hold it.
const Timeout = time.Minute

// ErrNotFound is returned when the command is not installed.
var ErrNotFound = errors.New("command not found")

// Find returns the path of the command, looked up in PATH then in the extra
// directories, and empty when none has it.
func Find(name string, dirs ...string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, dir := range dirs {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}

// Output runs the command found as Find finds it, and returns its standard
// output, with ErrNotFound when the command is not installed.
func Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return OutputIn(ctx, "", nil, name, args...)
}

// OutputIn is Output with the command run in dir, and found in PATH then in
// the extra directories.
func OutputIn(ctx context.Context, dir string, dirs []string, name string, args ...string) ([]byte, error) {
	p := Find(name, dirs...)
	if p == "" {
		return nil, ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p, args...)
	cmd.Dir = dir
	return cmd.Output()
}
