package capexec

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/opensvc/om3/util/pg"
	"github.com/opensvc/om3/util/sizeconv"
	"github.com/opensvc/om3/util/ulimit"
	"github.com/opensvc/om3/util/usergroup"
	"github.com/spf13/pflag"
)

type (
	T struct {
		PGID            *string
		PGCpus          *string
		PGMems          *string
		PGCPUShares     *string
		PGCPUQuota      *string
		PGMemOOMControl *string
		PGMemLimit      *string
		PGVMemLimit     *string
		PGMemSwappiness *string
		PGBlockIOWeight *string
		LimitAS         *string
		LimitCPU        *string
		LimitCore       *string
		LimitData       *string
		LimitFSize      *string
		LimitMemLock    *string
		LimitNoFile     *string
		LimitNProc      *string
		LimitRSS        *string
		LimitStack      *string
		LimitVMem       *string
		User            *string
		Group           *string
	}
)

func (t T) Argv() []string {
	argv := make([]string, 0)
	if t.PGID != nil {
		argv = append(argv, "--pg", *t.PGID)
	}
	if t.PGCpus != nil {
		argv = append(argv, "--pg-cpus", *t.PGCpus)
	}
	if t.PGMems != nil {
		argv = append(argv, "--pg-mems", *t.PGMems)
	}
	if t.PGCPUShares != nil {
		argv = append(argv, "--pg-cpu-shares", *t.PGCPUShares)
	}
	if t.PGCPUQuota != nil {
		argv = append(argv, "--pg-cpu-quota", *t.PGCPUQuota)
	}
	if t.PGMemOOMControl != nil {
		argv = append(argv, "--pg-mem-oom-control", *t.PGMemOOMControl)
	}
	if t.PGMemLimit != nil {
		argv = append(argv, "--pg-mem-limit", *t.PGMemLimit)
	}
	if t.PGVMemLimit != nil {
		argv = append(argv, "--pg-vmem-limit", *t.PGVMemLimit)
	}
	if t.PGMemSwappiness != nil {
		argv = append(argv, "--pg-mem-swappiness", *t.PGMemSwappiness)
	}
	if t.PGBlockIOWeight != nil {
		argv = append(argv, "--pg-blkio-weight", *t.PGBlockIOWeight)
	}
	if t.LimitAS != nil {
		argv = append(argv, "--limit-as", *t.LimitAS)
	}
	if t.LimitCPU != nil {
		argv = append(argv, "--limit-cpu", *t.LimitCPU)
	}
	if t.LimitCore != nil {
		argv = append(argv, "--limit-core", *t.LimitCore)
	}
	if t.LimitData != nil {
		argv = append(argv, "--limit-data", *t.LimitData)
	}
	if t.LimitFSize != nil {
		argv = append(argv, "--limit-fsize", *t.LimitFSize)
	}
	if t.LimitMemLock != nil {
		argv = append(argv, "--limit-memlock", *t.LimitMemLock)
	}
	if t.LimitNoFile != nil {
		argv = append(argv, "--limit-nofile", *t.LimitNoFile)
	}
	if t.LimitNProc != nil {
		argv = append(argv, "--limit-nproc", *t.LimitNProc)
	}
	if t.LimitRSS != nil {
		argv = append(argv, "--limit-rss", *t.LimitRSS)
	}
	if t.LimitStack != nil {
		argv = append(argv, "--limit-stack", *t.LimitStack)
	}
	if t.LimitVMem != nil {
		argv = append(argv, "--limit-vmem", *t.LimitVMem)
	}
	if t.User != nil {
		argv = append(argv, "--user", *t.User)
	}
	if t.Group != nil {
		argv = append(argv, "--group", *t.Group)
	}
	return argv
}

func (t T) toLimit() ulimit.Config {
	limit := ulimit.Config{}
	if t.LimitAS != nil {
		if i, err := sizeconv.FromSize(*t.LimitAS); err == nil {
			limit.AS = &i
		}
	}
	if t.LimitCPU != nil {
		if i, err := time.ParseDuration(*t.LimitCPU); err == nil {
			limit.CPU = &i
		}
	}
	if t.LimitCore != nil {
		if i, err := sizeconv.FromSize(*t.LimitCore); err == nil {
			limit.Core = &i
		}
	}
	if t.LimitData != nil {
		if i, err := sizeconv.FromSize(*t.LimitData); err == nil {
			limit.Data = &i
		}
	}
	if t.LimitFSize != nil {
		if i, err := sizeconv.FromSize(*t.LimitFSize); err == nil {
			limit.FSize = &i
		}
	}
	if t.LimitMemLock != nil {
		if i, err := sizeconv.FromSize(*t.LimitMemLock); err == nil {
			limit.MemLock = &i
		}
	}
	if t.LimitNoFile != nil {
		if i, err := sizeconv.FromSize(*t.LimitNoFile); err == nil {
			limit.NoFile = &i
		}
	}
	if t.LimitNProc != nil {
		if i, err := sizeconv.FromSize(*t.LimitNProc); err == nil {
			limit.NProc = &i
		}
	}
	if t.LimitRSS != nil {
		if i, err := sizeconv.FromSize(*t.LimitRSS); err == nil {
			limit.RSS = &i
		}
	}
	if t.LimitStack != nil {
		if i, err := sizeconv.FromSize(*t.LimitStack); err == nil {
			limit.Stack = &i
		}
	}
	if t.LimitVMem != nil {
		if i, err := sizeconv.FromSize(*t.LimitVMem); err == nil {
			limit.VMem = &i
		}
	}
	return limit
}

func (t T) toPG() pg.Config {
	pg := pg.Config{}
	if t.PGID != nil {
		pg.ID = *t.PGID
	}
	if t.PGCpus != nil {
		pg.CPUs = *t.PGCpus
	}
	if t.PGMems != nil {
		pg.Mems = *t.PGMems
	}
	if t.PGCPUShares != nil {
		pg.CPUShares = *t.PGCPUShares
	}
	if t.PGCPUQuota != nil {
		pg.CPUQuota = *t.PGCPUQuota
	}
	if t.PGMemOOMControl != nil {
		pg.MemOOMControl = *t.PGMemOOMControl
	}
	if t.PGMemLimit != nil {
		pg.MemLimit = *t.PGMemLimit
	}
	if t.PGVMemLimit != nil {
		pg.VMemLimit = *t.PGVMemLimit
	}
	if t.PGMemSwappiness != nil {
		pg.MemSwappiness = *t.PGMemSwappiness
	}
	if t.PGBlockIOWeight != nil {
		pg.BlockIOWeight = *t.PGBlockIOWeight
	}
	return pg
}

func (t *T) LoadLimit(g ulimit.Config) {
	if g.AS != nil {
		v := sizeconv.ExactDSizeCompact(float64(*g.AS))
		t.LimitAS = &v
	}
	if g.CPU != nil {
		v := g.CPU.String()
		t.LimitCPU = &v
	}
	if g.Core != nil {
		v := sizeconv.ExactDSizeCompact(float64(*g.Core))
		t.LimitCore = &v
	}
	if g.Data != nil {
		v := sizeconv.ExactDSizeCompact(float64(*g.Data))
		t.LimitData = &v
	}
	if g.FSize != nil {
		v := sizeconv.ExactDSizeCompact(float64(*g.FSize))
		t.LimitFSize = &v
	}
	if g.MemLock != nil {
		v := sizeconv.ExactDSizeCompact(float64(*g.MemLock))
		t.LimitMemLock = &v
	}
	if g.NoFile != nil {
		v := sizeconv.ExactDSizeCompact(float64(*g.NoFile))
		t.LimitNoFile = &v
	}
	if g.NProc != nil {
		v := sizeconv.ExactDSizeCompact(float64(*g.NProc))
		t.LimitNProc = &v
	}
	if g.RSS != nil {
		v := sizeconv.ExactDSizeCompact(float64(*g.RSS))
		t.LimitRSS = &v
	}
	if g.Stack != nil {
		v := sizeconv.ExactDSizeCompact(float64(*g.Stack))
		t.LimitStack = &v
	}
	if g.VMem != nil {
		v := sizeconv.ExactDSizeCompact(float64(*g.VMem))
		t.LimitVMem = &v
	}
}

func (t *T) LoadPG(g pg.Config) {
	if g.ID != "" {
		t.PGID = &g.ID
	}
	if g.CPUs != "" {
		t.PGCpus = &g.CPUs
	}
	if g.Mems != "" {
		t.PGMems = &g.Mems
	}
	if g.CPUShares != "" {
		t.PGCPUShares = &g.CPUShares
	}
	if g.CPUQuota != "" {
		t.PGCPUQuota = &g.CPUQuota
	}
	if g.MemOOMControl != "" {
		t.PGMemOOMControl = &g.MemOOMControl
	}
	if g.MemLimit != "" {
		t.PGMemLimit = &g.MemLimit
	}
	if g.VMemLimit != "" {
		t.PGVMemLimit = &g.VMemLimit
	}
	if g.MemSwappiness != "" {
		t.PGMemSwappiness = &g.MemSwappiness
	}
	if g.BlockIOWeight != "" {
		t.PGBlockIOWeight = &g.BlockIOWeight
	}
}

func (t *T) FlagSet(flags *pflag.FlagSet) {
	t.PGID = flags.String("pg", "", "the process group to attach to")
	t.PGCpus = flags.String("pg-cpus", "", "the cpus to pin the process group to (ex: 1-3,5)")
	t.PGMems = flags.String("pg-mems", "", "the memories to pin the process group to (ex: 1-3,5)")
	t.PGCPUShares = flags.String("pg-cpu-shares", "", "the cpu shares granted to the process group to (ex: 100)")
	t.PGCPUQuota = flags.String("pg-cpu-quota", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	t.PGMemOOMControl = flags.String("pg-mem-oom-control", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	t.PGMemLimit = flags.String("pg-mem-limit", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	t.PGVMemLimit = flags.String("pg-vmem-limit", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	t.PGMemSwappiness = flags.String("pg-mem-swappiness", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	t.PGBlockIOWeight = flags.String("pg-blkio-weight", "", "the cpu hardcap limit (in usecs). allowed cpu time in a given period")
	t.LimitAS = flags.String("limit-as", "", "the maximum area (in bytes) of address space which may be taken by the process")
	t.LimitCPU = flags.String("limit-cpu", "", "the maximum amount of processor time (in seconds) that a process can use")
	t.LimitCore = flags.String("limit-core", "", "the maximum size (in bytes) of a core file that the current process can create")
	t.LimitData = flags.String("limit-data", "", "the maximum size (in bytes) of the process’s heap")
	t.LimitFSize = flags.String("limit-fsize", "", "the maximum size of a file which the process may create")
	t.LimitMemLock = flags.String("limit-memlock", "", "the maximum address space which may be locked in memory")
	t.LimitNoFile = flags.String("limit-nofile", "", "the maximum number of open file descriptors for the current process")
	t.LimitNProc = flags.String("limit-nproc", "", "the maximum number of processes the current process may create")
	t.LimitRSS = flags.String("limit-rss", "", "the maximum resident set size that should be made available to the process")
	t.LimitStack = flags.String("limit-stack", "", "the maximum size (in bytes) of the call stack for the current process")
	t.LimitVMem = flags.String("limit-vmem", "", "the largest area of mapped memory which the process may occupy")
	t.User = flags.String("user", "", "execute the command as user")
	t.Group = flags.String("group", "", "execute the command as group")
}

func (t T) demote() error {
	if t.Group != nil && *t.Group != "" {
		gid, err := usergroup.GIDFromString(*t.Group)
		if err != nil {
			return err
		}
		err = syscall.Setgid(int(gid))
		if err != nil {
			return err
		}
	}
	if t.User != nil && *t.User != "" {
		uid, err := usergroup.UIDFromString(*t.User)
		if err != nil {
			return err
		}
		err = syscall.Setuid(int(uid))
		if err != nil {
			return err
		}
	}
	return nil
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
	if err := t.toLimit().Apply(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if t.PGID != nil && *t.PGID != "" {
		if err := t.toPG().Apply(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := t.demote(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	syscall.Exec(prog, args, os.Environ())
}
