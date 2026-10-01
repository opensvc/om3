package om

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
)

// The exit status of the shell an enter command ran is the exit status of
// the command, and is not printed as an error of it. An error that is not
// the status of the shell is printed as any other.
func TestQuietExitStatus(t *testing.T) {
	shellErr := exec.Command("/bin/sh", "-c", "exit 7").Run()
	if shellErr == nil {
		t.Fatal("the shell did not exit with a status")
	}
	for _, tc := range []struct {
		name      string
		err       error
		wantQuiet bool
	}{
		{"the status of the shell", shellErr, true},
		{"the status of the shell, as the object and the resource report it", fmt.Errorf("ns1/svc/web: %w", fmt.Errorf("container#1: %w", shellErr)), true},
		{"an error entering", errors.New("the container is not running"), false},
		{"no error", nil, false},
	} {
		cmd := &cobra.Command{}
		got := quietExitStatus(cmd, tc.err)
		if got != tc.err {
			t.Errorf("%s: the error was changed: %v", tc.name, got)
		}
		if cmd.SilenceErrors != tc.wantQuiet {
			t.Errorf("%s: silenced %v, want %v", tc.name, cmd.SilenceErrors, tc.wantQuiet)
		}
	}
	// The status is still what the command exits with.
	var coder interface{ ExitCode() int }
	if !errors.As(fmt.Errorf("container#1: %w", shellErr), &coder) || coder.ExitCode() != 7 {
		t.Errorf("the exit status of the shell is lost")
	}
}
