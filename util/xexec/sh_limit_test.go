package xexec

import (
	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/util/limits"
	"runtime"
	"testing"
	"time"
)

func TestT_shLimitCommands(t *testing.T) {
	var LimitNProcCommand string
	if runtime.GOOS == "darwin" {
		LimitNProcCommand = "ulimit -u 8"
	} else {
		LimitNProcCommand = "ulimit -p 8"
	}
	cases := map[string]struct {
		limit    limits.T
		commands []string
	}{
		"null": {
			limits.T{},
			[]string{},
		},
		"limit_nofile_64": {
			limits.T{LimitNoFile: 64},
			[]string{"ulimit -n 64"},
		},
		"limit_vmem_greater_than_as": {
			limits.T{LimitAs: 32000, LimitVMem: 64000},
			[]string{"ulimit -v 64"},
		},
		"limit_as_greater_than_limit_vmem": {
			limits.T{LimitAs: 64000, LimitVMem: 32000},
			[]string{"ulimit -v 64"},
		},
		"all_limits": {
			limits.T{
				LimitCpu:     2 * time.Hour,
				LimitCore:    3 * 512,
				LimitData:    4 * 1000,
				LimitFSize:   5 * 512,
				LimitMemLock: 6 * 1000,
				LimitNoFile:  7,
				LimitNProc:   8,
				LimitRss:     9 * 1000,
				LimitStack:   10 * 1000,
				LimitVMem:    11 * 1000,
			},
			[]string{
				"ulimit -n 7",
				"ulimit -s 10",
				"ulimit -l 6",
				"ulimit -v 11",
				"ulimit -t 7200",
				"ulimit -c 3",
				"ulimit -d 4",
				"ulimit -f 5",
				"ulimit -m 9",
				LimitNProcCommand,
			},
		},
	}
	for name := range cases {
		t.Run(name, func(t *testing.T) {
			commands := shLimitCommands(cases[name].limit)
			assert.ElementsMatch(t, cases[name].commands, commands)
		})
	}
}
