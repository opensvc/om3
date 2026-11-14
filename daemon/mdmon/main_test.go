package mdmon

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
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
		if _, ok := m.taker(name); ok {
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
		if _, ok := m.taker(name); ok {
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

// An array held by a resource that does not know its kernel name, as one
// assembled outside of the agent for a resource down, is held by its uuid,
// from its udev link, and its change published with it.
func TestManagerTellsTheTakerByUUID(t *testing.T) {
	m, _ := newTestManager(t)
	m.uuidOfKernelName = func(name string) string {
		if name == "md125" {
			return "569109e3:33b8d30c:12ee6eb1:08a1dee6"
		}
		return ""
	}
	m.counter.Set("svc1", []string{"uuid:569109e3:33b8d30c:12ee6eb1:08a1dee6"})
	uuid, ok := m.taker("md125")
	assert.True(t, ok)
	assert.Equal(t, "569109e3:33b8d30c:12ee6eb1:08a1dee6", uuid)
	_, ok = m.taker("md124")
	assert.False(t, ok, "an array of another uuid")

	// A kernel name a stale status still names, another array reused, is
	// published with the uuid of the array, for its holder by uuid.
	m.counter.Set("svc2", []string{"md125"})
	uuid, ok = m.taker("md125")
	assert.True(t, ok)
	assert.Equal(t, "569109e3:33b8d30c:12ee6eb1:08a1dee6", uuid)
}

// The counter keys of the md resources: the kernel name of their array, else
// its uuid, else none.
func TestKeysOf(t *testing.T) {
	st := instance.Status{Resources: instance.ResourceStatuses{
		"disk#1": resource.Status{Type: "disk.md", Info: map[string]any{"dev": "md127", "uuid": "a:b:c:d"}},
		"disk#2": resource.Status{Type: "disk.md", Info: map[string]any{"uuid": "e:f:g:h"}},
		"disk#3": resource.Status{Type: "disk.md"},
		"disk#4": resource.Status{Type: "disk.drbd", Info: map[string]any{"dev": "md1"}},
	}}
	assert.ElementsMatch(t, []string{"md127", "uuid:e:f:g:h", ""}, keysOf(st))
}
