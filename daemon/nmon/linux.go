//go:build linux

package nmon

import (
	"os"
	"strings"
)

// osBootedWithOpensvcFreeze returns true if os has been booted with opensvc frozen
//
// On Linux: boot command must contain osvc.freeze
func osBootedWithOpensvcFreeze() bool {
	b, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return false
	}
	cmdLine := strings.TrimSuffix(string(b), "\n")
	cmdLineArgs := strings.Split(cmdLine, " ")
	search := "osvc.freeze"
	for _, arg := range cmdLineArgs {
		if arg == search {
			return true
		}
	}
	return false
}
