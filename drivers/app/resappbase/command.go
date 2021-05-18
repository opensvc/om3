package resappbase

import (
	"os/exec"
	"strings"
)

func Command(command string) *exec.Cmd {
	commandSplit := strings.Split(command, " ")
	name := commandSplit[0]
	arg := commandSplit[1:]
	return exec.Command(name, arg...)
}
