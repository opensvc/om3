package scheduler

import (
	"container/heap"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/schedule"
)

type (
	// job is a schedule entry planned to run at its NextRunAt.
	job struct {
		entry     schedule.Entry
		createdAt time.Time

		// index is the position of the job in the queue, -1 when the job
		// is out of it, which it is from the moment it is due until it is
		// planned again.
		index int
	}

	// jobQueue orders the planned jobs by the time they are due, the
	// soonest first.
	jobQueue []*job

	// jobs holds the planned jobs, by object and schedule key, and in the
	// order they are due.
	//
	// It is owned by the scheduler loop, which alone reads and writes it:
	// there is no lock, and no timer of its own firing on another goroutine.
	jobs struct {
		byPath map[naming.Path]map[string]*job
		queue  jobQueue
	}
)

func (q jobQueue) Len() int {
	return len(q)
}

func (q jobQueue) Less(i, j int) bool {
	return q[i].entry.NextRunAt.Before(q[j].entry.NextRunAt)
}

func (q jobQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *jobQueue) Push(x any) {
	j := x.(*job)
	j.index = len(*q)
	*q = append(*q, j)
}

func (q *jobQueue) Pop() any {
	old := *q
	n := len(old)
	j := old[n-1]
	old[n-1] = nil
	j.index = -1
	*q = old[:n-1]
	return j
}

func newJobs() jobs {
	return jobs{
		byPath: make(map[naming.Path]map[string]*job),
	}
}

func (t *jobs) get(path naming.Path, key string) (*job, bool) {
	j, ok := t.byPath[path][key]
	return j, ok
}

func (t *jobs) has(path naming.Path, key string) bool {
	_, ok := t.get(path, key)
	return ok
}

// set plans the entry to run at its NextRunAt, in place of the job planned
// for its key, if any.
func (t *jobs) set(e schedule.Entry, now time.Time) {
	j, ok := t.get(e.Path, e.Key)
	if !ok {
		j = &job{createdAt: now, index: -1}
		if t.byPath[e.Path] == nil {
			t.byPath[e.Path] = make(map[string]*job)
		}
		t.byPath[e.Path][e.Key] = j
	}
	j.entry = e
	if j.index < 0 {
		heap.Push(&t.queue, j)
	} else {
		heap.Fix(&t.queue, j.index)
	}
}

func (t *jobs) del(path naming.Path, key string) {
	j, ok := t.get(path, key)
	if !ok {
		return
	}
	if j.index >= 0 {
		heap.Remove(&t.queue, j.index)
	}
	delete(t.byPath[path], key)
	if len(t.byPath[path]) == 0 {
		delete(t.byPath, path)
	}
}

func (t *jobs) delPath(path naming.Path) {
	for key := range t.byPath[path] {
		t.del(path, key)
	}
}

func (t *jobs) purge() {
	t.byPath = make(map[naming.Path]map[string]*job)
	t.queue = nil
}

// keys returns the schedule keys of the jobs of the object.
func (t *jobs) keys(path naming.Path) []string {
	l := make([]string, 0, len(t.byPath[path]))
	for key := range t.byPath[path] {
		l = append(l, key)
	}
	return l
}

// table returns the entries of the jobs of the object, with the times they
// last ran and are next due.
func (t *jobs) table(path naming.Path) schedule.Table {
	table := make(schedule.Table, 0, len(t.byPath[path]))
	for _, j := range t.byPath[path] {
		table = append(table, j.entry)
	}
	return table
}

// first returns the job due the soonest, nil when none is planned.
func (t *jobs) first() *job {
	if len(t.queue) == 0 {
		return nil
	}
	return t.queue[0]
}

// popDue takes the jobs due at now out of the queue, the soonest first.
// They stay known by their key, and are planned again by whoever takes
// them.
func (t *jobs) popDue(now time.Time) []*job {
	var l []*job
	for {
		j := t.first()
		if j == nil || j.entry.NextRunAt.After(now) {
			return l
		}
		heap.Pop(&t.queue)
		l = append(l, j)
	}
}

// len returns the number of jobs known.
func (t *jobs) len() int {
	n := 0
	for _, m := range t.byPath {
		n += len(m)
	}
	return n
}
