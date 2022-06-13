//go:build linux

package md

import "os/exec"

const (
	mdadm string = "/sbin/mdadm"
)

func IsCapable() bool {
	if _, err := exec.LookPath(mdadm); err != nil {
		return false
	}
	return true
}
