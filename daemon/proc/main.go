package proc

import (
	"sort"
	"sync"
)

type (
	// T is a process the daemon started and has not reaped.
	//
	// It holds the process facts and nothing else. What the process is
	// running - the object, the command, the origin, the resource, when it
	// began - is what the exec store remembers, under the same exec id, for
	// as long after it ends as the store keeps it. This is only how to find
	// it and how to signal it.
	T struct {
		Pid int

		// ExecID names the run this process is, which is what joins it to
		// what the exec store remembers of the same run. A session id would
		// not: one command reaching several objects of a node is several
		// processes sharing one session id.
		ExecID string
	}
)

var (
	mu    sync.RWMutex
	byPID = make(map[int]T)
)

func Register(t T) {
	if t.Pid <= 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	byPID[t.Pid] = t
}

func Unregister(pid int) {
	mu.Lock()
	defer mu.Unlock()
	delete(byPID, pid)
}

func Get(pid int) (T, bool) {
	mu.RLock()
	defer mu.RUnlock()
	t, ok := byPID[pid]
	return t, ok
}

// List returns every process the daemon started and has not reaped, by
// ascending pid. Narrowing is the exec store's job: it knows what each of
// these is running.
func List() []T {
	mu.RLock()
	out := make([]T, 0, len(byPID))
	for _, t := range byPID {
		out = append(out, t)
	}
	mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].Pid < out[j].Pid
	})
	return out
}

// PidByExecID indexes the live processes by the exec each one is running, so
// a listing of execs can say which of them still has a process and what its
// pid is.
func PidByExecID() map[string]int {
	mu.RLock()
	defer mu.RUnlock()
	out := make(map[string]int, len(byPID))
	for pid, t := range byPID {
		if t.ExecID != "" {
			out[t.ExecID] = pid
		}
	}
	return out
}
