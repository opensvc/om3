package mdmon

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/evwatch"
	"github.com/opensvc/om3/v3/util/plog"
)

// notifications records the debounced notifications of a manager.
type notifications struct {
	mu    sync.Mutex
	names []string
}

func (t *notifications) add(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.names = append(t.names, name)
}

// settled returns the names notified, once the debouncer delay passed.
func (t *notifications) settled() []string {
	time.Sleep(50 * time.Millisecond)
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string{}, t.names...)
}

// newTestManager returns a manager whose published changes are recorded,
// the taker of each told when it is published, as the manager does.
func newTestManager(t *testing.T) (*Manager, *notifications) {
	t.Helper()
	n := &notifications{}
	m := &Manager{log: plog.NewDefaultLogger(), counter: evwatch.NewCounter()}
	m.debouncer = evwatch.NewDebouncer(10*time.Millisecond, func(name string) {
		if m.isTaken(name) {
			n.add(name)
		}
	})
	t.Cleanup(m.debouncer.Stop)
	return m, n
}

// The change of an array is published when a local instance holds it at
// publish time, one of no array named when a local instance holds an md
// resource, nothing else.
func TestManagerPublishesTheChangesTaken(t *testing.T) {
	m, n := newTestManager(t)
	m.onEvent("md127")
	m.onEvent("")
	assert.Empty(t, n.settled(), "no md resource")

	m.counter.Set("svc1", []string{"md127"})
	m.onEvent("md126")
	assert.Empty(t, n.settled(), "an array no instance holds")
	m.onEvent("md127")
	m.onEvent("")
	assert.ElementsMatch(t, []string{"md127", ""}, n.settled())

	// An md resource of no array known yet takes no named change.
	m, n = newTestManager(t)
	m.counter.Set("svc2", []string{""})
	m.onEvent("md126")
	assert.Empty(t, n.settled())
}

// An array an instance just started is held by it at publish time, its
// status naming it by then, though not when the event was read.
func TestManagerTellsTheTakerAtPublishTime(t *testing.T) {
	n := &notifications{}
	m := &Manager{log: plog.NewDefaultLogger(), counter: evwatch.NewCounter()}
	m.debouncer = evwatch.NewDebouncer(100*time.Millisecond, func(name string) {
		if m.isTaken(name) {
			n.add(name)
		}
	})
	t.Cleanup(m.debouncer.Stop)
	m.counter.Set("svc1", []string{""})
	m.onEvent("md127")
	m.counter.Set("svc1", []string{"md127"})
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, []string{"md127"}, n.settled())
}
