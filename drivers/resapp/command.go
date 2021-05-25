package resapp

import (
	"os/exec"
)

func Command(command []string) *exec.Cmd {
	if len(command) == 0 {
		return nil
	}
	return exec.Command(command[0], command[1:]...)
}
