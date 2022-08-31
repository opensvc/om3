//go:build linux

package systemd

import "os"

var (
	procOneComm = "/proc/1/comm"
)

// HasSystemd return true if systemd is detected on current os
func HasSystemd() bool {
	var (
		b   []byte
		err error
	)
	if b, err = os.ReadFile(procOneComm); err != nil {
		return false
	}
	return string(b) == "systemd\n"
}
