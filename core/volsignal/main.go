package volsignal

import (
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type (
	T map[syscall.Signal]map[string]interface{}
)

func Parse(s string) T {
	t := make(map[syscall.Signal]map[string]interface{})
	for _, e := range strings.Fields(s) {
		l := strings.SplitN(e, ":", 2)
		if len(l) != 2 {
			continue
		}
		sigName := strings.ToUpper(l[0])
		if !strings.HasPrefix(sigName, "SIG") {
			sigName = "SIG" + sigName
		}
		sigNum := unix.SignalNum(sigName)
		if sigNum == 0 {
			continue
		}
		if _, ok := t[sigNum]; !ok {
			t[sigNum] = make(map[string]interface{})
		}
		for _, rid := range strings.Split(l[1], ",") {
			t[sigNum][rid] = nil
		}
	}
	return t
}
