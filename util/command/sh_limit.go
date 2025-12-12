package command

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/opensvc/om3/v3/util/limits"
)

// ShLimitCommands provides ulimit commands for sh launcher
// max value of LimitVMem, LimitAs is used to set virtual memory limit
func ShLimitCommands(l limits.T) string {
	commands := make([]string, 0)
	if l.LimitNoFile > 0 {
		// -n set the limit on the number files a process can have open at once
		commands = append(commands, "ulimit -n "+fmt.Sprintf("%d", l.LimitNoFile))
	}
	if l.LimitStack > 0 {
		// -s the limit on the stack size of a process (in kilobytes)
		commands = append(commands, "ulimit -s "+fmt.Sprintf("%d", l.LimitStack/1024))
	}
	if l.LimitMemLock > 0 && runtime.GOOS != "solaris" {
		// -l the limit on how much memory a process can lock with mlock(2) (in kilobytes)
		commands = append(commands, "ulimit -l "+fmt.Sprintf("%d", l.LimitMemLock/1024))
	}
	if l.LimitNProc > 0 && runtime.GOOS != "solaris" {
		var flag string
		if runtime.GOOS == "darwin" {
			flag = "-u"
		} else {
			// -p set the limit on the number of processes this user can have at one time
			flag = "-p"
		}
		commands = append(commands, fmt.Sprintf("ulimit %s %d", flag, l.LimitNProc))
	}
	if l.LimitVMem > 0 && l.LimitVMem >= l.LimitAS {
		// -v set the limit on the total virtual memory that can be in use by a process (in kilobytes)
		commands = append(commands, "ulimit -v "+fmt.Sprintf("%d", l.LimitVMem/1024))
	}
	if l.LimitAS > 0 && l.LimitAS > l.LimitVMem {
		// -v set the limit on the total virtual memory that can be in use by a process (in kilobytes)
		commands = append(commands, "ulimit -v "+fmt.Sprintf("%d", l.LimitAS/1024))
	}

	if l.LimitCPU > 0 {
		// -t show or set the limit on CPU time (in seconds)
		limitCPUSecond := int64(l.LimitCPU.Seconds())
		commands = append(commands, "ulimit -t "+fmt.Sprintf("%d", limitCPUSecond))
	}
	if l.LimitCore > 0 {
		// -c the limit on the largest core dump size that can be produced (in 512-byte blocks)
		commands = append(commands, "ulimit -c "+fmt.Sprintf("%d", l.LimitCore/512))
	}
	if l.LimitData > 0 {
		// -d show or set the limit on the data segment size of a process (in kilobytes)
		commands = append(commands, "ulimit -d "+fmt.Sprintf("%d", l.LimitData/1024))
	}
	if l.LimitFSize > 0 {
		// -f the limit on the largest file that can be created (in 512-byte blocks)
		commands = append(commands, "ulimit -f "+fmt.Sprintf("%d", l.LimitFSize/512))
	}
	if l.LimitRSS > 0 {
		// -m the limit on the total physical memory that can be in use by a process
		// (in kilobytes)
		commands = append(commands, "ulimit -m "+fmt.Sprintf("%d", l.LimitRSS/1024))
	}
	return strings.Join(commands, " && ")
}
