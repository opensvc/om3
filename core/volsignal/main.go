package volsignal

import (
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type (
	T map[syscall.Signal][]string
)

func Parse(s string) T {
	t := make(map[syscall.Signal][]string)
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
		rids := strings.Split(l[1], ",")
		t[sigNum] = rids
	}
	return T(t)
}
