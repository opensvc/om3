package capexec

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/pflag"
	"opensvc.com/opensvc/util/pg"
	"opensvc.com/opensvc/util/ulimit"
)

type T struct {
	PG    pg.Config
	Limit ulimit.Config
}

func (t T) Argv() []string {
	argv := make([]string, 0)
	if t.PG.ID != "" {
		argv = append(argv, "--pg", t.PG.ID)
	}
	if t.PG.Cpus != "" {
		argv = append(argv, "--pg-cpus", t.PG.Cpus)
	}
	if t.PG.Mems != "" {
		argv = append(argv, "--pg-mems", t.PG.Mems)
	}
	if t.PG.CpuShares != "" {
		argv = append(argv, "--pg-cpu-shares", t.PG.CpuShares)
	}
	if t.PG.CpuQuota != "" {
		argv = append(argv, "--pg-cpu-quota", t.PG.CpuQuota)
	}
	if t.PG.MemOOMControl != "" {
		argv = append(argv, "--pg-mem-oom-control", t.PG.MemOOMControl)
	}
	if t.PG.MemLimit != "" {
		argv = append(argv, "--pg-mem-limit", t.PG.MemLimit)
	}
	if t.PG.VMemLimit != "" {
		argv = append(argv, "--pg-vmem-limit", t.PG.VMemLimit)
	}
	if t.PG.MemSwappiness != "" {
		argv = append(argv, "--pg-mem-swappiness", t.PG.MemSwappiness)
	}
	if t.PG.BlkioWeight != "" {
		argv = append(argv, "--pg-blkio-weight", t.PG.BlkioWeight)
	}
	if t.Limit.AS != "" {
		argv = append(argv, "--limit-as", t.Limit.AS)
	}
	if t.Limit.CPU != "" {
		argv = append(argv, "--limit-cpu", t.Limit.CPU)
	}
	if t.Limit.Core != "" {
		argv = append(argv, "--limit-core", t.Limit.Core)
	}
	if t.Limit.Data != "" {
		argv = append(argv, "--limit-data", t.Limit.Data)
	}
	if t.Limit.FSize != "" {
		argv = append(argv, "--limit-fsize", t.Limit.FSize)
	}
	if t.Limit.MemLock != "" {
		argv = append(argv, "--limit-memlock", t.Limit.MemLock)
	}
	if t.Limit.NoFile != "" {
		argv = append(argv, "--limit-nofile", t.Limit.NoFile)
	}
	if t.Limit.NProc != "" {
		argv = append(argv, "--limit-nproc", t.Limit.NProc)
	}
	if t.Limit.RSS != "" {
		argv = append(argv, "--limit-rss", t.Limit.RSS)
	}
	if t.Limit.Stack != "" {
		argv = append(argv, "--limit-stack", t.Limit.Stack)
	}
	if t.Limit.VMem != "" {
		argv = append(argv, "--limit-vmem", t.Limit.VMem)
	}
	return argv
}

func (t *T) FlagSet(flags *pflag.FlagSet) {
	flags.StringVar(&t.PG.ID, "pg", "", "the process group to attach to")
	flags.StringVar(&t.PG.Cpus, "pg-cpus", "", "the cpus to pin the process group to (ex: 1-3,5)")
	flags.StringVar(&t.PG.Mems, "pg-mems", "", "the memories to pin the process group to (ex: 1-3,5)")
	flags.StringVar(&t.PG.CpuShares, "pg-cpu-shares", "", "the cpu shares granted to the process group to (ex: 100)")
	flags.StringVar(&t.PG.CpuQuota, "pg-cpu-quota", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&t.PG.MemOOMControl, "pg-mem-oom-control", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&t.PG.MemLimit, "pg-mem-limit", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&t.PG.VMemLimit, "pg-vmem-limit", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&t.PG.MemSwappiness, "pg-mem-swappiness", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&t.PG.BlkioWeight, "pg-blkio-weight", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&t.Limit.AS, "limit-as", "", "the maximum area (in bytes) of address space which may be taken by the process")
	flags.StringVar(&t.Limit.CPU, "limit-cpu", "", "the maximum amount of processor time (in seconds) that a process can use")
	flags.StringVar(&t.Limit.Core, "limit-core", "", "the maximum size (in bytes) of a core file that the current process can create")
	flags.StringVar(&t.Limit.Data, "limit-data", "", "the maximum size (in bytes) of the process’s heap")
	flags.StringVar(&t.Limit.FSize, "limit-fsize", "", "the maximum size of a file which the process may create")
	flags.StringVar(&t.Limit.MemLock, "limit-memlock", "", "the maximum address space which may be locked in memory")
	flags.StringVar(&t.Limit.NoFile, "limit-nofile", "", "the maximum number of open file descriptors for the current process")
	flags.StringVar(&t.Limit.NProc, "limit-nproc", "", "the maximum number of processes the current process may create")
	flags.StringVar(&t.Limit.RSS, "limit-rss", "", "the maximum resident set size that should be made available to the process")
	flags.StringVar(&t.Limit.Stack, "limit-stack", "", "the maximum size (in bytes) of the call stack for the current process")
	flags.StringVar(&t.Limit.VMem, "limit-vmem", "", "the largest area of mapped memory which the process may occupy")
}

func (t T) Exec(args []string) {
	//fmt.Printf("options: %+v\n", t)
	//fmt.Printf("args: %+v\n", args)
	if len(args) == 0 {
		return
	}
	prog, err := exec.LookPath(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := t.Limit.Apply(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := t.PG.Apply(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	syscall.Exec(prog, args, os.Environ())
}
