package command

import (
	"github.com/stretchr/testify/assert"
	"github.com/opensvc/om3/util/limits"
	"runtime"
	"testing"
	"time"
)

func TestT_shLimitCommands(t *testing.T) {
	type testCase struct {
		limit    limits.T
		commands string
	}
	cases := map[string]testCase{
		"null": {
			limits.T{},
			"",
		},
		"limit_nofile_64": {
			limits.T{LimitNoFile: 64},
			"ulimit -n 64",
		},
		"limit_vmem_greater_than_as": {
			limits.T{LimitAs: 2048000, LimitVMem: 4096000},
			"ulimit -v 4000",
		},
		"limit_as_greater_than_limit_vmem": {
			limits.T{LimitAs: 4096000, LimitVMem: 2048000},
			"ulimit -v 4000",
		},
		"all_limits": {
			limits.T{
				LimitCpu:    2 * time.Hour,
				LimitCore:   3 * 512,
				LimitData:   4 * 1024,
				LimitFSize:  5 * 512,
				LimitNoFile: 7,
				LimitRss:    9 * 1024,
				LimitStack:  10 * 1024,
				LimitVMem:   11 * 1024,
			},
			"ulimit -n 7" +
				" && ulimit -s 10" +
				" && ulimit -v 11" +
				" && ulimit -t 7200" +
				" && ulimit -c 3" +
				" && ulimit -d 4" +
				" && ulimit -f 5" +
				" && ulimit -m 9",
		},
	}
	if runtime.GOOS == "darwin" {
		cases["limit_nproc"] = testCase{
			limits.T{LimitNProc: 8},
			"ulimit -u 8",
		}
		cases["limit_memlock"] = testCase{
			limits.T{LimitMemLock: 6 * 1024},
			"ulimit -l 6",
		}
	} else if runtime.GOOS == "linux" {
		cases["limit_nproc"] = testCase{
			limits.T{LimitNProc: 8},
			"ulimit -p 8",
		}
		cases["limit_memlock"] = testCase{
			limits.T{LimitMemLock: 6 * 1024},
			"ulimit -l 6",
		}
	}
	for name := range cases {
		t.Run(name, func(t *testing.T) {
			commands := ShLimitCommands(cases[name].limit)
			assert.Equal(t, cases[name].commands, commands)
		})
	}
}
