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
