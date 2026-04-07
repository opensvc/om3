package proc

import (
	"slices"
	"sort"
	"sync"
	"time"
)

type (
	T struct {
		Pid          int
		Node         string
		Object       string
		Sid          string
		StartedAt    time.Time
		Elapsed      string
		GlobalExpect string
		Sub          string
		Desc         string
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
	if t.StartedAt.IsZero() {
		t.StartedAt = time.Now()
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
	t, ok := byPID[pid]
	mu.RUnlock()
	if !ok {
		return T{}, false
	}
	if !t.StartedAt.IsZero() {
		d := time.Since(t.StartedAt)
		if d < 0 {
			d = 0
		}
		t.Elapsed = d.String()
	}
	return t, true
}

func List(subFilters []string) []T {
	mu.RLock()
	out := make([]T, 0, len(byPID))
	for _, t := range byPID {
		if len(subFilters) != 0 && !slices.Contains(subFilters, t.Sub) {
			continue
		}
		if !t.StartedAt.IsZero() {
			d := time.Since(t.StartedAt)
			if d < 0 {
				d = 0
			}
			t.Elapsed = d.String()
		}
		out = append(out, t)
	}
	mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].Pid < out[j].Pid
	})
	return out
}
