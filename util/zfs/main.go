package zfs

import "os/exec"

func IsCapable() bool {
	if _, err := exec.LookPath("/usr/sbin/zfs"); err != nil {
		return false
	}
	return true
}
