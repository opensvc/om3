package scsi

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/plog"
)

// fakeLun is a lun as SPC-3 keeps its persistent reservation: registrations
// and the reservation belong to a nexus, a path. The node reaches it by its
// current paths, which a restart can change, as the initiator session ids of
// iscsi paths are given in login order.
type fakeLun struct {
	paths        []string
	registered   map[string]string // nexus -> key
	holder       string            // nexus holding the reservation
	holderKey    string
	releaseCalls int

	// preempts are the service actions of the preempts, "preempt" or
	// "preempt-abort".
	preempts []string

	// onRelease, if set, runs before a release, as a peer taking the lun
	// meanwhile.
	onRelease func()

	// unregisterErr, if set, fails the unregistrations.
	unregisterErr error
}

var errConflict = errors.New("reservation conflict")

func newFakeLun(paths ...string) *fakeLun {
	return &fakeLun{paths: paths, registered: make(map[string]string)}
}

func (t *fakeLun) keys() []string {
	l := make([]string, 0)
	for _, k := range t.registered {
		l = append(l, k)
	}
	slices.Sort(l)
	return l
}

func (t *fakeLun) ReadRegistrations(device.T) ([]string, error) { return t.keys(), nil }
func (t *fakeLun) ReadReservation(device.T) (string, error)     { return t.holderKey, nil }

// Register is a register and ignore existing key, from every current path.
func (t *fakeLun) Register(_ device.T, key string) error {
	for _, p := range t.paths {
		t.registered[p] = key
	}
	return nil
}

// Unregister is a register and ignore existing key to zero, from every
// current path: a path not registered is left as it is.
func (t *fakeLun) Unregister(_ device.T, _ string) error {
	if t.unregisterErr != nil {
		return t.unregisterErr
	}
	for _, p := range t.paths {
		delete(t.registered, p)
		if t.holder == p {
			t.holder, t.holderKey = "", ""
		}
	}
	return nil
}

func (t *fakeLun) Reserve(_ device.T, key string) error {
	for _, p := range t.paths {
		if t.registered[p] != key {
			continue
		}
		if t.holder != "" && t.holder != p {
			return errConflict
		}
		t.holder, t.holderKey = p, key
		return nil
	}
	return errConflict
}

// Release from a path not registered is a conflict, from a registered path
// not holding the reservation does nothing, and from the holder releases.
func (t *fakeLun) Release(_ device.T, key string) error {
	t.releaseCalls++
	if t.onRelease != nil {
		t.onRelease()
	}
	for _, p := range t.paths {
		if t.registered[p] != key {
			continue
		}
		if t.holder == p {
			t.holder, t.holderKey = "", ""
		}
		return nil
	}
	return errConflict
}

func (t *fakeLun) Clear(device.T, string) error { return errors.New("not expected") }

// Preempt, from the first current path registered with newKey, removes the
// registrations of oldKey but its own, and takes the reservation if oldKey
// holds it.
func (t *fakeLun) Preempt(dev device.T, oldKey, newKey string) error {
	t.preempts = append(t.preempts, "preempt")
	return t.preempt(oldKey, newKey)
}

func (t *fakeLun) preempt(oldKey, newKey string) error {
	issuer := ""
	for _, p := range t.paths {
		if t.registered[p] == newKey {
			issuer = p
			break
		}
	}
	if issuer == "" {
		return errConflict
	}
	for n, k := range t.registered {
		if k == oldKey && n != issuer {
			delete(t.registered, n)
		}
	}
	if t.holderKey == oldKey {
		t.holder, t.holderKey = issuer, newKey
	}
	return nil
}

// PreemptAbort is a preempt also aborting the commands the preempted paths
// have pending, which the fake has none of.
func (t *fakeLun) PreemptAbort(_ device.T, oldKey, newKey string) error {
	t.preempts = append(t.preempts, "preempt-abort")
	return t.preempt(oldKey, newKey)
}

const (
	ourKey     = "0x0000000000000001"
	foreignKey = "0x0000000000000002"
)

func newHandle(t *testing.T, lun *fakeLun) *PersistentReservationHandle {
	t.Helper()
	p := filepath.Join(t.TempDir(), "lun")
	require.NoError(t, os.WriteFile(p, nil, 0o600))
	return &PersistentReservationHandle{
		Key:                         ourKey,
		Devices:                     device.L{device.New(p)},
		Log:                         plog.NewDefaultLogger(),
		StatusLogger:                discardStatus{},
		persistentReservationDriver: lun,
		expectedRegistrationCount:   func(device.T) (int, error) { return len(lun.paths), nil },
	}
}

type discardStatus struct{}

func (discardStatus) Info(string, ...any)  {}
func (discardStatus) Warn(string, ...any)  {}
func (discardStatus) Error(string, ...any) {}

// restarted is a lun the node registered and reserved through the paths a
// and b, then reaches through c and d after it restarted.
func restarted() *fakeLun {
	lun := newFakeLun("a", "b")
	_ = lun.Register(device.T{}, ourKey)
	_ = lun.Reserve(device.T{}, ourKey)
	lun.paths = []string{"c", "d"}
	return lun
}

// A stop after a restart releases the reservation the node holds from a path
// of before, and leaves no registration of its key: the release from paths
// not registered was refused, and the stop and the unprovision failed.
func TestAStopAfterARestartReleasesTheReservation(t *testing.T) {
	lun := restarted()
	require.NoError(t, newHandle(t, lun).Stop())
	assert.Empty(t, lun.holderKey, "released")
	assert.Empty(t, lun.registered, "no registration left, of the paths of before or of now")
}

// A stop with the paths registered releases and unregisters, as before.
func TestAStopReleasesTheReservation(t *testing.T) {
	lun := newFakeLun("a", "b")
	_ = lun.Register(device.T{}, ourKey)
	_ = lun.Reserve(device.T{}, ourKey)
	require.NoError(t, newHandle(t, lun).Stop())
	assert.Empty(t, lun.holderKey)
	assert.Empty(t, lun.registered)
}

// A stop leaves a reservation another key holds, and registers no path: a
// registration would give the node write access to a lun another node holds.
func TestAStopLeavesAForeignReservation(t *testing.T) {
	lun := newFakeLun("x")
	_ = lun.Register(device.T{}, foreignKey)
	_ = lun.Reserve(device.T{}, foreignKey)
	lun.registered["a"] = ourKey // a path of before
	lun.paths = []string{"c", "d"}
	require.NoError(t, newHandle(t, lun).Stop())
	assert.Equal(t, foreignKey, lun.holderKey)
	assert.Equal(t, "x", lun.holder)
	assert.NotContains(t, lun.registered, "c", "no path registered under a foreign reservation")
	assert.NotContains(t, lun.registered, "d")
	assert.Equal(t, 0, lun.releaseCalls, "no release tried")
}

// A stop of a lun not reserved drops the registrations of paths of before.
func TestAStopDropsTheRegistrationsOfPathsOfBefore(t *testing.T) {
	lun := newFakeLun("a", "b")
	_ = lun.Register(device.T{}, ourKey)
	lun.paths = []string{"c", "d"}
	require.NoError(t, newHandle(t, lun).Stop())
	assert.Empty(t, lun.registered)
}

// A stop of a lun neither reserved nor registered does nothing.
func TestAStopOfALunUnusedDoesNothing(t *testing.T) {
	lun := newFakeLun("a")
	require.NoError(t, newHandle(t, lun).Stop())
	assert.Equal(t, 0, lun.releaseCalls)
}

// A start after a restart moves the reservation to a current path and drops
// the registrations of the paths of before: the reservation stayed held by a
// path of before, the registrations in excess warned for good, and a stop
// then left the lun reserved.
func TestAStartAfterARestartMovesTheReservationToACurrentPath(t *testing.T) {
	lun := restarted()
	require.NoError(t, newHandle(t, lun).Start())
	assert.Equal(t, ourKey, lun.holderKey)
	assert.Contains(t, []string{"c", "d"}, lun.holder, "held by a current path")
	assert.Equal(t, map[string]string{"c": ourKey, "d": ourKey}, lun.registered, "the current paths registered, the ones of before dropped")
}

// A start never preempts a reservation another key holds unless forced, as
// before.
func TestAStartRefusesAForeignReservation(t *testing.T) {
	lun := newFakeLun("x")
	_ = lun.Register(device.T{}, foreignKey)
	_ = lun.Reserve(device.T{}, foreignKey)
	lun.paths = []string{"c"}
	err := newHandle(t, lun).Start()
	assert.ErrorContains(t, err, "already reserved by the foreign key")
	assert.Equal(t, "x", lun.holder)
}

// A stop during which a peer takes the lun, its preempt removing the
// registrations of our key, is done: the release and the unregistration
// failing then, the lun is reserved by another key and holds no registration
// of ours, which is what the stop was to reach.
func TestAStopIsDoneWhenAPeerTookTheLunMeanwhile(t *testing.T) {
	lun := newFakeLun("a", "b")
	_ = lun.Register(device.T{}, ourKey)
	_ = lun.Reserve(device.T{}, ourKey)
	lun.onRelease = func() {
		for n, k := range lun.registered {
			if k == ourKey {
				delete(lun.registered, n)
			}
		}
		lun.registered["x"] = foreignKey
		lun.holder, lun.holderKey = "x", foreignKey
		lun.unregisterErr = errConflict
	}
	require.NoError(t, newHandle(t, lun).Stop())
	assert.Equal(t, foreignKey, lun.holderKey)

	// The same, with a registration of ours left: the stop fails.
	lun = newFakeLun("a")
	_ = lun.Register(device.T{}, ourKey)
	_ = lun.Reserve(device.T{}, ourKey)
	lun.onRelease = func() {
		lun.holder, lun.holderKey = "x", foreignKey
		lun.unregisterErr = errConflict
	}
	assert.ErrorContains(t, newHandle(t, lun).Stop(), "spr unregister")
}

// The own key is preempted as another key is, with an abort, unless
// no_preempt_abort says otherwise.
func TestTheOwnKeyIsPreemptedWithAnAbort(t *testing.T) {
	lun := restarted()
	require.NoError(t, newHandle(t, lun).Stop())
	assert.Equal(t, []string{"preempt-abort"}, lun.preempts)

	lun = restarted()
	h := newHandle(t, lun)
	h.NoPreemptAbort = true
	require.NoError(t, h.Start())
	assert.Equal(t, []string{"preempt"}, lun.preempts)
}
