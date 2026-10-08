//go:build linux

package toc

import (
	"os"
)

// sysrqTrigger is the file a SysRq command is written to. Writing it is
// allowed to root whatever kernel.sysrq says, as that setting is for the
// keyboard only (Documentation/admin-guide/sysrq.rst).
const sysrqTrigger = "/proc/sysrq-trigger"

// sysrq runs the SysRq command of the character c.
func sysrq(c byte) error {
	return os.WriteFile(sysrqTrigger, []byte{c}, 0o200)
}

// Reboot reboots the node at once, without syncing nor unmounting the disks.
func Reboot() error {
	return sysrq('b')
}

// Crash crashes the kernel, which takes a crash dump when one is configured.
func Crash() error {
	return sysrq('c')
}
