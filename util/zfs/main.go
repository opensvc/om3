package zfs

import "os/exec"

func IsCapable() bool {
	if _, err := exec.LookPath("zfs"); err != nil {
		return false
	}
	return true
}
