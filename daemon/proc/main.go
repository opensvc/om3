package proc

import (
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
