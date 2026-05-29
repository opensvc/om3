//go:build linux

package md

import "os/exec"

func IsCapable() bool {
	if _, err := exec.LookPath(mdadm); err != nil {
		return false
	}
	if _, err := exec.LookPath(blkid); err != nil {
		return false
	}
	return true
}
