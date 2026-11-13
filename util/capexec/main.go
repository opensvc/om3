package capexec

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/pflag"

	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/ulimit"
	"github.com/opensvc/om3/v3/util/usergroup"
)

type (
	T struct {
		PGID            *string
		PGCpus          *string
		PGMems          *string
		PGCPUShares     *string
		PGCPUQuota      *string
		PGCPUBurst      *string
		PGMemOOMControl *string
		PGMemLimit      *string
		PGMemHigh       *string
		PGVMemLimit     *string
		PGMemSwappiness *string
		PGBlockIOWeight *string
		PGPidsMax       *string
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

		// Umask is the umask of the command, in octal, as 022. Not set,
		// the command inherits the one of om exec.
		Umask *string
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
	if t.PGCPUBurst != nil {
		argv = append(argv, "--pg-cpu-burst", *t.PGCPUBurst)
	}
	if t.PGMemHigh != nil {
		argv = append(argv, "--pg-mem-high", *t.PGMemHigh)
	}
	if t.PGPidsMax != nil {
		argv = append(argv, "--pg-pids-max", *t.PGPidsMax)
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
	if t.Umask != nil {
		argv = append(argv, "--umask", *t.Umask)
	}
	return argv
}

// toLimit returns the limits of the flags, and an error naming each flag
// whose value is not one: a limit ignored is one the user believes applied.
func (t T) toLimit() (ulimit.Config, error) {
	limit := ulimit.Config{}
	var errs error
	size := func(flag string, v *string, dst **int64) {
		if v == nil || *v == "" {
			return
		}
		i, err := sizeconv.FromSize(*v)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("--%s %q: %w", flag, *v, err))
			return
		}
		*dst = &i
	}
	size("limit-as", t.LimitAS, &limit.AS)
	if t.LimitCPU != nil && *t.LimitCPU != "" {
		if i, err := time.ParseDuration(*t.LimitCPU); err != nil {
			errs = errors.Join(errs, fmt.Errorf("--limit-cpu %q: %w", *t.LimitCPU, err))
		} else {
			limit.CPU = &i
		}
	}
	size("limit-core", t.LimitCore, &limit.Core)
	size("limit-data", t.LimitData, &limit.Data)
	size("limit-fsize", t.LimitFSize, &limit.FSize)
	size("limit-memlock", t.LimitMemLock, &limit.MemLock)
	size("limit-nofile", t.LimitNoFile, &limit.NoFile)
	size("limit-nproc", t.LimitNProc, &limit.NProc)
	size("limit-rss", t.LimitRSS, &limit.RSS)
	size("limit-stack", t.LimitStack, &limit.Stack)
	size("limit-vmem", t.LimitVMem, &limit.VMem)
	return limit, errs
}

// pgFlags returns the --pg-* flags set, which cap the process group --pg
// names.
func (t T) pgFlags() []string {
	l := make([]string, 0)
	for _, f := range []struct {
		name string
		v    *string
	}{
		{"pg-cpus", t.PGCpus},
		{"pg-mems", t.PGMems},
		{"pg-cpu-shares", t.PGCPUShares},
		{"pg-cpu-quota", t.PGCPUQuota},
		{"pg-cpu-burst", t.PGCPUBurst},
		{"pg-mem-oom-control", t.PGMemOOMControl},
		{"pg-mem-limit", t.PGMemLimit},
		{"pg-mem-high", t.PGMemHigh},
		{"pg-vmem-limit", t.PGVMemLimit},
		{"pg-mem-swappiness", t.PGMemSwappiness},
		{"pg-blkio-weight", t.PGBlockIOWeight},
		{"pg-pids-max", t.PGPidsMax},
	} {
		if f.v != nil && *f.v != "" {
			l = append(l, "--"+f.name)
		}
	}
	return l
}

// hasPG says the command is to run in the process group --pg names.
func (t T) hasPG() bool {
	return t.PGID != nil && *t.PGID != ""
}

// checkPG refuses a --pg-* flag with no --pg: the cap would be ignored.
func (t T) checkPG() error {
	if l := t.pgFlags(); len(l) > 0 && !t.hasPG() {
		return fmt.Errorf("%s: a process group cap needs the process group, name it with --pg", strings.Join(l, " "))
	}
	return nil
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
	if t.PGCPUBurst != nil {
		pg.CPUBurst = *t.PGCPUBurst
	}
	if t.PGMemHigh != nil {
		pg.MemHigh = *t.PGMemHigh
	}
	if t.PGPidsMax != nil {
		pg.PidsMax = *t.PGPidsMax
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
	if g.CPUBurst != "" {
		t.PGCPUBurst = &g.CPUBurst
	}
	if g.MemHigh != "" {
		t.PGMemHigh = &g.MemHigh
	}
	if g.PidsMax != "" {
		t.PGPidsMax = &g.PidsMax
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
	t.PGID = flags.String("pg", "", "the process group to run the command in, as /opensvc.slice/opensvc-ns.test.slice/opensvc-ns.test-svc.web.slice, which the --pg-* flags cap")
	t.PGCpus = flags.String("pg-cpus", "", "the cpus the process group is pinned to, as 1-3,5")
	t.PGMems = flags.String("pg-mems", "", "the memory nodes the process group is pinned to, as 0-1")
	t.PGCPUShares = flags.String("pg-cpu-shares", "", "the cpu shares of the process group, relative to its siblings, as 100")
	t.PGCPUQuota = flags.String("pg-cpu-quota", "", "the cpu time the process group may use, as 50% of one cpu, or 100%@4 for 4 cpus")
	t.PGCPUBurst = flags.String("pg-cpu-burst", "", "the cpu time the process group may use above its quota in a period, as 25%@4 (unified hierarchy only)")
	t.PGMemOOMControl = flags.String("pg-mem-oom-control", "", "1 to disable the oom killer of the process group (v1 hierarchy only)")
	t.PGMemLimit = flags.String("pg-mem-limit", "", "the memory the process group may use, as 512m")
	t.PGMemHigh = flags.String("pg-mem-high", "", "the memory above which the process group is throttled and reclaimed, as 384m (unified hierarchy only)")
	t.PGVMemLimit = flags.String("pg-vmem-limit", "", "the memory and swap the process group may use, as 1g")
	t.PGMemSwappiness = flags.String("pg-mem-swappiness", "", "the swappiness of the process group, from 0 to 100 (v1 hierarchy only)")
	t.PGBlockIOWeight = flags.String("pg-blkio-weight", "", "the block io weight of the process group, relative to its siblings, as 500")
	t.PGPidsMax = flags.String("pg-pids-max", "", "the number of processes the process group may hold, as 100")
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
	t.Umask = flags.String("umask", "", "the umask of the command, in octal, as 022")
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

// ReadyFDEnv names the variable of the descriptor a caller of Exec waits on
// for the command to take over: the descriptor is closed by the execve of
// the command, and the error is written to it when the command can not be
// executed. The command inherits neither the descriptor nor the variable.
//
// It is the protocol a Go program starting a command uses with its child, one
// level deeper: the caller learns the command runs, in its cgroup, with its
// limits and its identity, without guessing a delay.
const ReadyFDEnv = "OPENSVC_EXEC_READY_FD"

// NotReady reports err on the descriptor ReadyFDEnv names, if any, to the
// caller waiting for the command to take over: a refusal of the arguments
// before Exec, as a command missing, is reported there too.
func NotReady(err error) {
	if ready := readyFile(); ready != nil {
		_, _ = ready.WriteString(err.Error())
		_ = ready.Close()
	}
}

// ErrNoCommand is the error of an exec given no command to run.
var ErrNoCommand = errors.New("a command to run is required, after --")

// readyFile returns the descriptor named by ReadyFDEnv, closed on exec, and
// unsets the variable.
func readyFile() *os.File {
	s, ok := os.LookupEnv(ReadyFDEnv)
	if !ok {
		return nil
	}
	_ = os.Unsetenv(ReadyFDEnv)
	fd, err := strconv.Atoi(s)
	if err != nil || fd < 3 {
		return nil
	}
	syscall.CloseOnExec(fd)
	return os.NewFile(uintptr(fd), "ready")
}

// setUmask sets the umask the command inherits, as v2 did before running
// it. A umask of 0 is set too: it is how a configuration asks for files
// readable and writable by all, which only an unset umask leaves to the
// one inherited.
func (t T) setUmask() error {
	if t.Umask == nil || *t.Umask == "" {
		return nil
	}
	i, err := strconv.ParseUint(*t.Umask, 8, 32)
	if err != nil || i > 0o777 {
		return fmt.Errorf("invalid umask %q: an octal number of at most 777", *t.Umask)
	}
	syscall.Umask(int(i))
	return nil
}

func (t T) Exec(args []string) {
	ready := readyFile()
	fail := func(err error) {
		fmt.Fprintln(os.Stderr, err)
		if ready != nil {
			_, _ = ready.WriteString(err.Error())
		}
		os.Exit(1)
	}
	if len(args) == 0 {
		fail(ErrNoCommand)
	}
	prog, err := exec.LookPath(args[0])
	if err != nil {
		fail(err)
	}
	if err := t.setUmask(); err != nil {
		fail(err)
	}
	if err := t.checkPG(); err != nil {
		fail(err)
	}
	if limit, err := t.toLimit(); err != nil {
		fail(err)
	} else if err := limit.Apply(); err != nil {
		fail(err)
	}
	if t.hasPG() {
		if err := t.toPG().Apply(); err != nil {
			fail(err)
		}
	}
	if err := t.demote(); err != nil {
		fail(err)
	}
	// returns only on failure: the command did not take over.
	err = syscall.Exec(prog, args, os.Environ())
	fail(fmt.Errorf("exec %s: %w", prog, err))
}
