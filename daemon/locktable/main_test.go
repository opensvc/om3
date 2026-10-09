package locktable

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rebuiltTable() *Table {
	t := NewTable()
	t.Rebuild(t.Generation(), nil)
	return t
}

// A table grants nothing before it was rebuilt from the nodes: what it holds
// is what it had when it last spoke for the cluster, or nothing.
func TestAcquireRefusedUntilRebuilt(t *testing.T) {
	table := NewTable()
	_, err := table.Acquire(context.Background(), Request{Name: "a"})
	assert.ErrorIs(t, err, ErrNotRebuilt)

	table.Rebuild(table.Generation(), nil)
	_, err = table.Acquire(context.Background(), Request{Name: "a"})
	assert.NoError(t, err)

	table.Drop()
	_, err = table.Acquire(context.Background(), Request{Name: "b"})
	assert.ErrorIs(t, err, ErrNotRebuilt)
}

func TestAcquireExcludes(t *testing.T) {
	table := rebuiltTable()
	ctx := context.Background()
	first, err := table.Acquire(ctx, Request{Name: "a", Node: "n1", Holder: "first"})
	require.NoError(t, err)

	_, err = table.Acquire(ctx, Request{Name: "a", Node: "n2"})
	var errHeld ErrHeld
	require.ErrorAs(t, err, &errHeld)
	assert.Equal(t, first.ID, errHeld.Lock.ID)

	_, err = table.Acquire(ctx, Request{Name: "b"})
	assert.NoError(t, err, "another name is another lock")

	_, ok := table.Release("a", "not-the-id")
	assert.False(t, ok)
	released, ok := table.Release("a", first.ID)
	assert.True(t, ok)
	assert.Equal(t, "n1", released.Node)
	_, err = table.Acquire(ctx, Request{Name: "a", Node: "n2"})
	assert.NoError(t, err)
}

// A request waiting for a lock is granted it when it is released.
func TestAcquireWaitsForTheRelease(t *testing.T) {
	table := rebuiltTable()
	ctx := context.Background()
	first, err := table.Acquire(ctx, Request{Name: "a"})
	require.NoError(t, err)

	var wg sync.WaitGroup
	var second Lock
	var secondErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		second, secondErr = table.Acquire(ctx, Request{Name: "a", Wait: 5 * time.Second})
	}()
	time.Sleep(50 * time.Millisecond)
	_, ok := table.Release("a", first.ID)
	require.True(t, ok)
	wg.Wait()
	require.NoError(t, secondErr)
	assert.NotEqual(t, first.ID, second.ID)
}

// The end of a lease frees the lock its holder never released.
func TestLeaseEndFreesTheLock(t *testing.T) {
	table := rebuiltTable()
	ctx := context.Background()
	_, err := table.Acquire(ctx, Request{Name: "a", Lease: 50 * time.Millisecond})
	require.NoError(t, err)
	_, err = table.Acquire(ctx, Request{Name: "a", Wait: 2 * time.Second})
	assert.NoError(t, err)
}

// A table dropped while requests wait answers them ErrNotRebuilt, so they
// are asked again of the node speaking now.
func TestDropAnswersTheWaiting(t *testing.T) {
	table := rebuiltTable()
	ctx := context.Background()
	_, err := table.Acquire(ctx, Request{Name: "a"})
	require.NoError(t, err)
	done := make(chan error)
	go func() {
		_, err := table.Acquire(ctx, Request{Name: "a", Wait: 5 * time.Second})
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	table.Drop()
	select {
	case err := <-done:
		assert.True(t, errors.Is(err, ErrNotRebuilt))
	case <-time.After(2 * time.Second):
		t.Fatal("the waiting request was not answered")
	}
}

// A rebuilt table holds what the nodes hold, and the lease of a lock two
// speakers granted apart is what settles it.
func TestRebuild(t *testing.T) {
	table := NewTable()
	now := time.Now()
	conflicts, ok := table.Rebuild(table.Generation(), []Lock{
		{Name: "a", ID: "1", ExpiresAt: now.Add(time.Minute)},
		{Name: "a", ID: "2", ExpiresAt: now.Add(2 * time.Minute)},
		{Name: "b", ID: "3", ExpiresAt: now.Add(-time.Second)},
	})
	require.True(t, ok)
	require.Len(t, conflicts, 1)
	assert.Equal(t, "1", conflicts[0].ID)
	l := table.List()
	require.Len(t, l, 1)
	assert.Equal(t, "2", l[0].ID, "an expired lock is not held")

	_, err := table.Acquire(context.Background(), Request{Name: "a"})
	var errHeld ErrHeld
	assert.ErrorAs(t, err, &errHeld)
	_, err = table.Acquire(context.Background(), Request{Name: "b"})
	assert.NoError(t, err)
}

func TestLease(t *testing.T) {
	assert.Equal(t, DefaultLease, Lease(0))
	assert.Equal(t, MaxLease, Lease(time.Hour))
	assert.Equal(t, time.Second, Lease(time.Second))
}

func TestHeld(t *testing.T) {
	held := NewHeld()
	now := time.Now()
	held.Add(Lock{Name: "b", ID: "2", ExpiresAt: now.Add(time.Minute)})
	held.Add(Lock{Name: "a", ID: "1", ExpiresAt: now.Add(time.Minute)})
	held.Add(Lock{Name: "c", ID: "3", ExpiresAt: now.Add(-time.Second)})
	l := held.List()
	require.Len(t, l, 2)
	assert.Equal(t, "a", l[0].Name)
	held.Remove("a", "other")
	assert.Len(t, held.List(), 2)
	held.Remove("a", "1")
	assert.Len(t, held.List(), 1)
}

// A rebuild started before the table was dropped does not mark it rebuilt:
// the nodes were read for a table this node may no longer keep.
func TestRebuildOfAnOlderGeneration(t *testing.T) {
	table := NewTable()
	generation := table.Generation()
	table.Drop()
	_, ok := table.Rebuild(generation, nil)
	assert.False(t, ok)
	assert.False(t, table.IsRebuilt())
}

// A table dropped keeps what it granted until the leases end, which is where
// a table rebuilt elsewhere finds a lock granted to a request handed over and
// not recorded yet by the node of its client.
func TestGrantedOutlivesTheDrop(t *testing.T) {
	table := rebuiltTable()
	ctx := context.Background()
	lock, err := table.Acquire(ctx, Request{Name: "a", Node: "n2"})
	require.NoError(t, err)
	_, err = table.Acquire(ctx, Request{Name: "b", Node: "n2", Lease: 30 * time.Millisecond})
	require.NoError(t, err)
	require.Len(t, table.Granted(), 2)

	table.Drop()
	assert.Empty(t, table.List())
	l := table.Granted()
	require.Len(t, l, 2, "the grants of the term outlive it")

	time.Sleep(50 * time.Millisecond)
	l = table.Granted()
	require.Len(t, l, 1, "until their lease ends")
	assert.Equal(t, lock.ID, l[0].ID)

	// Rebuilt, the table holds its former grant again, listed once.
	table.Rebuild(table.Generation(), l)
	assert.Len(t, table.Granted(), 1)

	// Released, it is gone from both.
	_, ok := table.Release("a", lock.ID)
	assert.True(t, ok)
	assert.Empty(t, table.Granted())

	// Forget drops a former grant released elsewhere.
	other, err := table.Acquire(ctx, Request{Name: "c", Node: "n3"})
	require.NoError(t, err)
	table.Drop()
	table.Forget("c", other.ID)
	assert.Empty(t, table.Granted())
}

// A release reaches the nodes one after the other, and a rebuild may read a
// node that has not heard of it yet. Forget clears the lock from the table
// such a rebuild filled, and keeps its id from the rebuilds that follow.
func TestForgetBeatsARebuildFromAStaleNode(t *testing.T) {
	now := time.Now()
	stale := Lock{Name: "a", ID: "1", Node: "n2", ExpiresAt: now.Add(time.Minute)}

	// The rebuild read the stale record before the release reached here.
	table := NewTable()
	table.Rebuild(table.Generation(), []Lock{stale})
	require.Len(t, table.List(), 1)
	table.Forget("a", "1")
	assert.Empty(t, table.List(), "the active lock is cleared")
	_, err := table.Acquire(context.Background(), Request{Name: "a"})
	assert.NoError(t, err, "and the lock is free again")

	// The release reached here before the rebuild read the stale record.
	table = NewTable()
	table.Forget("a", "1")
	table.Rebuild(table.Generation(), []Lock{stale})
	assert.Empty(t, table.List(), "a released id is not rebuilt")

	// A release on this table keeps the id from its next rebuild too.
	table = rebuiltTable()
	lock, err := table.Acquire(context.Background(), Request{Name: "b"})
	require.NoError(t, err)
	_, ok := table.Release("b", lock.ID)
	require.True(t, ok)
	table.Drop()
	table.Rebuild(table.Generation(), []Lock{lock})
	assert.Empty(t, table.List())
}
