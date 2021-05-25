package resapp

import (
	"os/exec"
)

func Command(command []string) *exec.Cmd {
	return exec.Command(command[0], command[1:]...)
}
