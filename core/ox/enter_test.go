package ox

import (
	"errors"
	"fmt"
	"testing"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/console"
)

// The exit status of the shell of a console session is the exit status of
// the enter command, and is not printed as an error of it. A session that
// could not be opened is an error printed as any other.
func TestQuietExitStatus(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		wantQuiet bool
	}{
		{"the status of the shell", console.ExitStatus(7), true},
		{"the status of the shell, wrapped", fmt.Errorf("ns1/svc/web: %w", console.ExitStatus(7)), true},
		{"a session that could not be opened", errors.New("the container is not running"), false},
		{"no error", nil, false},
	} {
		cmd := &cobra.Command{}
		if got := quietExitStatus(cmd, tc.err); got != tc.err {
			t.Errorf("%s: the error was changed: %v", tc.name, got)
		}
		if cmd.SilenceErrors != tc.wantQuiet {
			t.Errorf("%s: silenced %v, want %v", tc.name, cmd.SilenceErrors, tc.wantQuiet)
		}
	}
	var coder interface{ ExitCode() int }
	if !errors.As(error(console.ExitStatus(7)), &coder) || coder.ExitCode() != 7 {
		t.Error("the exit status of the shell is lost")
	}
}
