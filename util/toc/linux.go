//go:build linux

package toc

import (
	"github.com/mlafeldt/sysrq"
)

func Reboot() error {
	return sysrq.Trigger(sysrq.Reboot)
}

func Crash() error {
	return sysrq.Trigger(sysrq.Crash)
}
