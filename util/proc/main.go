package proc

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	p := t.Head() + "/environ"
	l, err := parseFile(p)
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
