//go:build linux

package raw

import "os/exec"

func IsCapable() bool {
	if _, err := exec.LookPath(raw); err != nil {
		return false
	}
	return true
}
