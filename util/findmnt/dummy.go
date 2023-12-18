//go:build !solaris

package findmnt

import "os/exec"

func mountCmd() *exec.Cmd {
	return exec.Command("mount")
}
