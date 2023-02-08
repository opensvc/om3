package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/daemon/routinehelper"
)

type (
	A struct {
		routinehelper.TT
	}
)

func New() *A {
	return &A{TT: *routinehelper.NewTracer()}
}

func (a *A) RunFooAndBar(nb int, running, done chan string) {
	for i := 0; i < nb; i++ {
		go func() {
			name := "foo"
			defer a.Trace(name)()
			running <- name
			done <- name
		}()
		go func() {
			name := "bar"
			defer a.Trace(name)()
			running <- name
			done <- name
		}()
	}
	return
}

func TestTracer(t *testing.T) {
	a := New()
	done := make(chan string)
	running := make(chan string)
	stat := a.TraceRDump()
	t.Logf("Initial stat details: %#v", stat)
	require.Equal(t, 0, stat.Count)

	routineToCreate := 5
	a.RunFooAndBar(routineToCreate, running, done)

	for i := 0; i < routineToCreate; i++ {
		t.Logf("now running %s", <-running)
	}
	stat = a.TraceRDump()
	t.Logf("half of routines are now running, details: %#v", stat)
	assert.GreaterOrEqualf(t, stat.Max, routineToCreate,
		"%d goroutines are running in //, but max is %d, details:%#v",
		routineToCreate, stat.Max, stat)
	assert.GreaterOrEqualf(t, stat.Count, routineToCreate,
		"2d goroutines are running in //, but count is %d, details:%#v",
		routineToCreate, stat.Count, stat)

	for i := 0; i < routineToCreate; i++ {
		t.Logf("now running %s", <-running)
	}
	stat = a.TraceRDump()
	t.Logf("All routine are now running, details: %#v", stat)
	assert.Equal(t, stat.Max, 2*routineToCreate,
		"2*%d goroutines are running in //, but max is %d, details:%#v",
		routineToCreate, stat.Max, stat)
	assert.Equal(t, stat.Count, 2*routineToCreate,
		"2*%d goroutines are running in //, but count is %d, details:%#v",
		routineToCreate, stat.Count, stat)

	for i := 0; i < routineToCreate*2; i++ {
		t.Logf("now done %s", <-done)
	}
	time.Sleep(1 * time.Millisecond)
	stat = a.TraceRDump()
	t.Logf("Now all routine are done: details: %#v", stat)
	assert.Equal(t, 2*routineToCreate, stat.Max,
		"expected 2*%d max, found %d, details:%#v",
		2*routineToCreate, stat.Max, stat)
	assert.Equal(t, 0, stat.Count,
		"0 goroutines are running in //, but count is %d, details:%#v",
		stat.Count, stat)
}
