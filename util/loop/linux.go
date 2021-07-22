// +build linux

package loop

import "os/exec"

func IsCapable() bool {
	if _, err := exec.LookPath(losetup); err != nil {
		return false
	}
	return true
}
