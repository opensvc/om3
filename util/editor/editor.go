package editor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/anmitsu/go-shlex"
)

// Edit runs the editor of the user on the file p, and returns when it exits.
//
// The editor is $EDITOR, split as a shell splits words, so an editor given
// with its arguments, as "code --wait" or "emacs -nw", runs as git, crontab
// or visudo run it. It is executed with no shell: the value is not evaluated.
func Edit(p string) error {
	argv, err := command(os.Getenv("EDITOR"))
	if err != nil {
		return err
	}
	cmd := exec.Command(argv[0], append(argv[1:], p)...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %s: %w", argv[0], err)
	}
	return nil
}

// command returns the argv of the editor value s, the editor of the system
// when s is empty.
func command(s string) ([]string, error) {
	if s == "" {
		switch runtime.GOOS {
		case "windows":
			return []string{"notepad.exe"}, nil
		default:
			return []string{"vi"}, nil
		}
	}
	argv, err := shlex.Split(s, true)
	if err != nil {
		return nil, fmt.Errorf("EDITOR %q: %w", s, err)
	}
	if len(argv) == 0 {
		return nil, fmt.Errorf("EDITOR %q names no command", s)
	}
	return argv, nil
}
