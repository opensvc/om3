package rescontainerkvm

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/claim"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/plog"
)

type fakeCapper struct {
	limit int64
	err   error
}

func (t fakeCapper) ResourceCap(string, string) (int64, error) { return t.limit, t.err }

// A node libvirt places the guests of out of the process groups is refused a
// guest the object caps, not only one the namespace claims for.
func TestCappedOn(t *testing.T) {
	what, err := cappedOn(nil, "container#1", claim.TypeMemory)
	assert.NoError(t, err)
	assert.Empty(t, what, "no object to ask")

	what, err = cappedOn(fakeCapper{limit: claim.Unbounded}, "container#1", claim.TypeMemory)
	assert.NoError(t, err)
	assert.Empty(t, what)

	what, err = cappedOn(fakeCapper{limit: 1024}, "container#1", claim.TypeCPU)
	assert.NoError(t, err)
	assert.Equal(t, "container#1 is capped on the "+claim.TypeCPU, what)

	failed := errors.New("failed")
	_, err = cappedOn(fakeCapper{err: failed}, "container#1", claim.TypeMemory)
	assert.ErrorIs(t, err, failed)
}

// fakeGuestObject is an object capping its guest and its namespace as told.
type fakeGuestObject struct {
	memCap       int64
	namespaceCap int64

	// pgCapKeyword is a pg_* keyword the configuration of the namespace
	// sets.
	pgCapKeyword string
}

func (t fakeGuestObject) NamespacePGCap() (string, error) { return t.pgCapKeyword, nil }

func (t fakeGuestObject) Log() *plog.Logger                                       { return plog.NewDefaultLogger() }
func (t fakeGuestObject) VarDir() string                                          { return "" }
func (t fakeGuestObject) ResourceByID(string) resource.Driver                     { return nil }
func (t fakeGuestObject) ResourcesByDrivergroups([]driver.Group) resource.Drivers { return nil }
func (t fakeGuestObject) MemCap(string) (int64, error)                            { return t.memCap, nil }
func (t fakeGuestObject) NamespaceMemCap() (int64, error)                         { return t.namespaceCap, nil }

// newCappedGuest returns a guest down of 256m, on a node placing guests in
// partitions, the object and the namespace capping it as told, with the file
// its fake virsh records its calls in.
func newCappedGuest(t *testing.T, memCap, namespaceCap int64) (*T, string) {
	t.Helper()
	_, callsFile := fakeVirsh(t, "shut off", "true", "true")
	previous := partitionCapable
	partitionCapable = func() bool { return true }
	t.Cleanup(func() { partitionCapable = previous })
	r := newStopTestT(t)
	r.Name = "om3-test-capped-guest"
	r.ResourceID = &resourceid.T{Name: "container#1"}
	memory := int64(256 << 20)
	r.Memory = &memory
	r.SetPG(&pg.Config{ID: testGuestGroup})
	r.SetObject(fakeGuestObject{memCap: memCap, namespaceCap: namespaceCap})
	return r, callsFile
}

// A guest of 256m needs 640m, its memory and what qemu takes. A cap below
// that, on its object or on its namespace, is the error its status says while
// it is down, and the start is refused on it before anything is set up: it
// went on, the guest running under a cap the first use of its memory hits.
func TestStartRefusesAGuestItsMemoryCapCanNotHold(t *testing.T) {
	need := int64(256<<20) + claim.GuestMemoryMargin
	cases := map[string]struct {
		memCap       int64
		namespaceCap int64
		want         string
	}{
		"the object caps below the need": {
			memCap:       512 << 20,
			namespaceCap: claim.Unbounded,
			want:         "memory cap 512mi: the guest needs 640mi, its memory and what qemu takes",
		},
		"the namespace caps below the need": {
			memCap:       claim.Unbounded,
			namespaceCap: 512 << 20,
			want:         "memory cap 512mi of the namespace: the guest needs 640mi, its memory and what qemu takes",
		},
		"the object cap holds the guest, the namespace one does not": {
			memCap:       need,
			namespaceCap: need - 1,
			want:         "of the namespace",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r, callsFile := newCappedGuest(t, tc.memCap, tc.namespaceCap)
			assert.ErrorContains(t, r.memCapError(), tc.want, "what the status and the provision say")
			assert.ErrorContains(t, r.Start(t.Context()), tc.want)
			assert.Equal(t, "dominfo", calls(t, callsFile), "the start asks the state of the guest and nothing else")
		})
	}
}

// The caps holding the guest, or none, refuse nothing.
func TestAMemoryCapHoldingTheGuestIsNoError(t *testing.T) {
	need := int64(256<<20) + claim.GuestMemoryMargin
	for name, caps := range map[string][2]int64{
		"no cap":             {claim.Unbounded, claim.Unbounded},
		"caps of the need":   {need, need},
		"caps over the need": {1 << 30, 2 << 30},
	} {
		t.Run(name, func(t *testing.T) {
			r, _ := newCappedGuest(t, caps[0], caps[1])
			assert.NoError(t, r.memCapError())
		})
	}
}

func (t fakeGuestObject) ResourceCap(string, string) (int64, error) { return t.memCap, nil }
