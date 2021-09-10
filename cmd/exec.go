package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/util/pg"
	"opensvc.com/opensvc/util/ulimit"
)

type execOpts struct {
	PG    pg.Config
	Limit ulimit.Config
}

var (
	execCmd = &cobra.Command{
		Use:   "exec",
		Short: "Execute a command with cappings and limits",
		Run: func(_ *cobra.Command, args []string) {
			runExec(args)
		},
	}
	xo execOpts
)

func init() {
	root.AddCommand(execCmd)
	flags := execCmd.Flags()
	flags.StringVar(&xo.PG.ID, "pg", "", "the process group to attach to")
	flags.StringVar(&xo.PG.Cpus, "pg-cpus", "", "the cpus to pin the process group to (ex: 1-3,5)")
	flags.StringVar(&xo.PG.Mems, "pg-mems", "", "the memories to pin the process group to (ex: 1-3,5)")
	flags.StringVar(&xo.PG.CpuShares, "pg-cpu-shares", "", "the cpu shares granted to the process group to (ex: 100)")
	flags.StringVar(&xo.PG.CpuQuota, "pg-cpu-quota", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&xo.PG.MemOOMControl, "pg-mem-oom-control", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&xo.PG.MemLimit, "pg-mem-limit", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&xo.PG.VMemLimit, "pg-vmem-limit", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&xo.PG.MemSwappiness, "pg-mem-swappiness", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&xo.PG.BlkioWeight, "pg-blkio-weight", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	flags.StringVar(&xo.Limit.AS, "limit-as", "", "the maximum area (in bytes) of address space which may be taken by the process")
	flags.StringVar(&xo.Limit.CPU, "limit-cpu", "", "the maximum amount of processor time (in seconds) that a process can use")
	flags.StringVar(&xo.Limit.Core, "limit-core", "", "the maximum size (in bytes) of a core file that the current process can create")
	flags.StringVar(&xo.Limit.Data, "limit-data", "", "the maximum size (in bytes) of the process’s heap")
	flags.StringVar(&xo.Limit.FSize, "limit-fsize", "", "the maximum size of a file which the process may create")
	flags.StringVar(&xo.Limit.MemLock, "limit-memlock", "", "the maximum address space which may be locked in memory")
	flags.StringVar(&xo.Limit.NoFile, "limit-nofile", "", "the maximum number of open file descriptors for the current process")
	flags.StringVar(&xo.Limit.NProc, "limit-nproc", "", "the maximum number of processes the current process may create")
	flags.StringVar(&xo.Limit.RSS, "limit-rss", "", "the maximum resident set size that should be made available to the process")
	flags.StringVar(&xo.Limit.Stack, "limit-stack", "", "the maximum size (in bytes) of the call stack for the current process")
	flags.StringVar(&xo.Limit.VMem, "limit-vmem", "", "the largest area of mapped memory which the process may occupy")
}

func runExec(args []string) {
	//fmt.Printf("options: %+v\n", xo)
	//fmt.Printf("args: %+v\n", args)
	if len(args) == 0 {
		return
	}
	prog, err := exec.LookPath(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := xo.Limit.Apply(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := xo.PG.Apply(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	syscall.Exec(prog, args, os.Environ())
}
