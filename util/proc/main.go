package proc

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/opensvc/om3/v3/util/stringslice"
)

type (
	T struct {
		pid int
		env map[string]string
	}
	L struct {
		procs []T
	}

	procStat struct {
		state       string
		flags       uint64
		numThreads  int
		envEndIsSet bool
	}
)

const (
	// pfKThread is the PF_KTHREAD bit of the /proc/<pid>/stat flags field.
	pfKThread = 0x00200000

	environRetryDelay       = time.Millisecond
	environRetryTimeout     = 100 * time.Millisecond
	environGoneRetryTimeout = 5 * time.Millisecond
)

var (
	sep = []byte{0x0}
)

func parseFile(p string) ([]string, error) {
	l := make([]string, 0)
	b, err := os.ReadFile(p)
	if err != nil {
		return l, err
	}
	b = bytes.TrimRightFunc(b, func(r rune) bool {
		return r == 0x0
	})
	for _, s := range bytes.Split(b, sep) {
		l = append(l, string(s))
	}
	return l, nil
}

func All() (L, error) {
	l := NewList()
	matches, err := filepath.Glob("/proc/*/cmdline")
	if err != nil {
		return l, err
	}
	for _, p := range matches {
		pidStr := filepath.Base(filepath.Dir(p))
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		l.AddPID(pid)
	}
	return l, nil
}

func ByCmdline(c []string) (L, error) {
	l := NewList()
	if len(c) == 0 {
		return l, nil
	}
	matches, err := filepath.Glob("/proc/*/cmdline")
	if err != nil {
		return l, err
	}
	for _, p := range matches {
		cmdline, err := parseFile(p)
		if err != nil {
			continue
		}
		if !stringslice.Equal(cmdline, c) {
			continue
		}
		pidStr := filepath.Base(filepath.Dir(p))
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		l.AddPID(pid)
	}
	return l, nil
}

func New(pid int) T {
	t := T{pid: pid}
	return t
}

func (t T) String() string {
	return fmt.Sprintf("%d", t.pid)
}

func (t T) PID() int {
	return t.pid
}

func (t T) Head() string {
	return fmt.Sprintf("/proc/%d", t.pid)
}

func (t T) Process() (*os.Process, error) {
	return os.FindProcess(t.pid)
}

func (t T) Signal(sig os.Signal) error {
	proc, err := t.Process()
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

func (t T) CommandLine() string {
	p := t.Head() + "/cmdline"
	l, err := parseFile(p)
	if err != nil {
		return ""
	}
	if len(l) == 0 {
		return ""
	}
	return l[0]
}

func (t *T) Env() map[string]string {
	if t.env != nil {
		return t.env
	}
	env := make(map[string]string)
	l, err := t.readEnviron()
	if err != nil {
		return env
	}
	for _, line := range l {
		words := strings.SplitN(line, "=", 2)
		if len(words) != 2 {
			continue
		}
		env[words[0]] = words[1]
	}
	t.env = env
	return env
}

// readEnviron returns the /proc/<pid>/environ entries.
//
// While a process execve's, its environ can't be read for up to a few
// milliseconds:
//
//   - the open fails with ESRCH, and the /proc/<pid> lookup can fail, while
//     the kernel makes the exec'ing thread the thread group leader, if the
//     execve was called from another thread (as Go programs do).
//   - the read is empty until the kernel has set the env bounds of the new
//     program memory.
//
// The read is retried while the process is in one of these states, so a
// process can still be recognized by its env while it exec's.
//
// Kernel threads, zombies and processes rewriting their env area to set
// their title (nginx, redis, ...) also can't be read, but for good, so
// their read is not retried with a delay.
func (t T) readEnviron() ([]string, error) {
	p := t.Head() + "/environ"
	start := time.Now()
	deadline := start.Add(environRetryTimeout)
	goneDeadline := start.Add(environGoneRetryTimeout)
	for {
		l, err := parseFile(p)
		if err == nil && !isEmpty(l) {
			return l, nil
		}
		if err != nil && !errors.Is(err, syscall.ESRCH) {
			return l, err
		}
		st, statErr := t.stat()
		switch {
		case errors.Is(statErr, syscall.ESRCH):
			// the stat open also fails while the exec'ing thread
			// becomes the thread group leader.
		case statErr != nil:
			// the process is gone, or the /proc/<pid> lookup fails
			// while the exec'ing thread becomes the thread group
			// leader. Retry for a shorter time, as a process reaped
			// between the environ and the stat reads gets here too.
			if time.Now().After(goneDeadline) {
				return l, err
			}
		case !st.mayBeExecuting(err != nil):
			if err == nil {
				// env_end may have been set between the environ
				// and the stat reads.
				return parseFile(p)
			}
			return l, err
		}
		if time.Now().After(deadline) {
			return l, err
		}
		time.Sleep(environRetryDelay)
	}
}

func isEmpty(l []string) bool {
	return len(l) == 0 || (len(l) == 1 && l[0] == "")
}

// stat returns the /proc/<pid>/stat fields telling if a process may be
// exec'ing.
func (t T) stat() (procStat, error) {
	var st procStat
	b, err := os.ReadFile(t.Head() + "/stat")
	if err != nil {
		return st, err
	}
	// the comm field is parenthesized and may contain spaces,
	// so split the fields after its closing parenthesis.
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return st, fmt.Errorf("%s/stat: no comm field end", t.Head())
	}
	// fields[0] is the field 3 of proc_pid_stat(5)
	fields := strings.Fields(string(b[i+1:]))
	if len(fields) < 49 {
		return st, fmt.Errorf("%s/stat: %d fields, want at least 51", t.Head(), len(fields)+2)
	}
	st.state = fields[0]
	if st.flags, err = strconv.ParseUint(fields[9-3], 10, 64); err != nil {
		return st, err
	}
	if st.numThreads, err = strconv.Atoi(fields[20-3]); err != nil {
		return st, err
	}
	st.envEndIsSet = fields[51-3] != "0"
	return st, nil
}

// mayBeExecuting returns true if the process state is consistent with an
// execve in progress, given its environ open failed with ESRCH (esrch
// true) or read empty (esrch false).
func (st procStat) mayBeExecuting(esrch bool) bool {
	if st.flags&pfKThread != 0 {
		return false
	}
	isZombie := st.state == "Z" || st.state == "X"
	if esrch {
		// the old leader of an exec'ing thread group shows as a zombie,
		// but the exec'ing thread still counts in the group.
		return !isZombie || st.numThreads > 1
	}
	if isZombie {
		return false
	}
	// the kernel reads an empty environ until env_end is set, which is
	// the last env bound the execve sets.
	return !st.envEndIsSet
}

func (t L) String() string {
	l := make([]string, t.Len())
	for i, p := range t.Procs() {
		l[i] = strconv.FormatInt(int64(p.pid), 10)
	}
	return fmt.Sprintf("pids[%s]", strings.Join(l, ","))

}

func (t L) Procs() []T {
	return t.procs
}

func (t L) Len() int {
	return len(t.procs)
}

func NewList() L {
	t := L{
		procs: make([]T, 0),
	}
	return t
}

func (t *L) AddPID(pid int) {
	t.Add(New(pid))
}

func (t L) HasPID(pid int) bool {
	for _, p := range t.procs {
		if p.pid == pid {
			return true
		}
	}
	return false
}

func (t *L) Add(proc T) {
	t.procs = append(t.procs, proc)
}

func (t *L) FilterByEnvList(keys []string, value string) L {
	l := NewList()
	for _, p := range t.Procs() {
		for _, key := range keys {
			v, ok := p.Env()[key]
			if !ok || v != value {
				continue
			}
			l.Add(p)
			break
		}
	}
	return l
}

func (t *L) FilterByEnv(key string, value string) L {
	l := NewList()
	for _, p := range t.Procs() {
		v, ok := p.Env()[key]
		if !ok || v != value {
			continue
		}
		l.Add(p)
	}
	return l
}

// PPID is the id of the parent process, read from /proc/<pid>/stat.
func (t T) PPID() (int, error) {
	b, err := os.ReadFile(t.Head() + "/stat")
	if err != nil {
		return 0, err
	}
	// the comm field is parenthesized and may contain spaces, so the
	// fields are split after its closing parenthesis: the state, then the
	// parent process id.
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return 0, fmt.Errorf("%s/stat: no comm field end", t.Head())
	}
	fields := strings.Fields(string(b[i+1:]))
	if len(fields) < 2 {
		return 0, fmt.Errorf("%s/stat: no ppid field", t.Head())
	}
	return strconv.Atoi(fields[1])
}

// Tree is pid and the ids of all its descendants, parents first, from one
// scan of /proc.
func Tree(pid int) []int {
	children := make(map[int][]int)
	matches, _ := filepath.Glob("/proc/[0-9]*/stat")
	for _, p := range matches {
		child, err := strconv.Atoi(filepath.Base(filepath.Dir(p)))
		if err != nil {
			continue
		}
		ppid, err := New(child).PPID()
		if err != nil {
			continue
		}
		children[ppid] = append(children[ppid], child)
	}
	l := []int{}
	queue := []int{pid}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		l = append(l, p)
		queue = append(queue, children[p]...)
	}
	return l
}
