//go:build linux

package systemd

import (
	"os"
)

// HasSystemd return true if systemd is detected on current os
func HasSystemd() bool {
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return false
	}
	return true
}
