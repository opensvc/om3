package uxid

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCaches(t *testing.T) {
	var calls atomic.Int32
	r := New(func(uid uint32) (Identity, error) {
		calls.Add(1)
		return Identity{UID: uid, Login: "alice", Groups: []string{"osvc-ns1"}}, nil
	})
	for i := 0; i < 3; i++ {
		id, err := r.Resolve(context.Background(), 1001)
		require.NoError(t, err)
		assert.Equal(t, "alice", id.Login)
	}
	assert.Equal(t, int32(1), calls.Load(), "one lookup, then the cache")
	r.Flush()
	_, _ = r.Resolve(context.Background(), 1001)
	assert.Equal(t, int32(2), calls.Load(), "looked up again once flushed")
}

func TestResolveCachesFailuresShortly(t *testing.T) {
	var calls atomic.Int32
	r := New(func(uid uint32) (Identity, error) {
		calls.Add(1)
		return Identity{}, errors.New("unknown uid")
	})
	r.negativeTTL = 50 * time.Millisecond
	_, err := r.Resolve(context.Background(), 1001)
	assert.Error(t, err)
	_, err = r.Resolve(context.Background(), 1001)
	assert.Error(t, err)
	assert.Equal(t, int32(1), calls.Load(), "the failure is cached")
	time.Sleep(60 * time.Millisecond)
	_, _ = r.Resolve(context.Background(), 1001)
	assert.Equal(t, int32(2), calls.Load(), "a short time")
}

// A name service not answering holds no caller past the deadline, and the
// lookups it holds are capped: the callers past the cap are refused at once.
func TestResolveBlockedNameService(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	r := New(func(uid uint32) (Identity, error) {
		<-release
		return Identity{UID: uid}, nil
	})
	r.timeout = 100 * time.Millisecond
	for uid := uint32(0); uid < DefaultMaxInFlight; uid++ {
		begin := time.Now()
		_, err := r.Resolve(context.Background(), 2000+uid)
		assert.ErrorIs(t, err, ErrTimeout)
		assert.Less(t, time.Since(begin), time.Second)
	}
	begin := time.Now()
	_, err := r.Resolve(context.Background(), 3000)
	assert.ErrorIs(t, err, ErrBusy)
	assert.Less(t, time.Since(begin), 50*time.Millisecond, "refused at once")

	// A caller of a uid already being looked up waits on that lookup,
	// taking no slot.
	_, err = r.Resolve(context.Background(), 2000)
	assert.ErrorIs(t, err, ErrTimeout)
}

// An identity is looked up again past its positive TTL: a group membership
// removed, or an account removed, is seen then.
func TestResolveSeesChangesSoon(t *testing.T) {
	var groups atomic.Value
	groups.Store([]string{"osvc-ns1"})
	r := New(func(uid uint32) (Identity, error) {
		return Identity{UID: uid, Login: "alice", Groups: groups.Load().([]string)}, nil
	})
	r.positiveTTL = 50 * time.Millisecond
	id, err := r.Resolve(context.Background(), 1001)
	require.NoError(t, err)
	assert.Equal(t, []string{"osvc-ns1"}, id.Groups)
	groups.Store([]string{})
	time.Sleep(60 * time.Millisecond)
	id, err = r.Resolve(context.Background(), 1001)
	require.NoError(t, err)
	assert.Empty(t, id.Groups, "the membership removed is seen")
}

// A name service not answering leaves the identities looked up lately as
// they were last seen, for a time, and the others refused.
func TestResolveKeepsKnownIdentitiesWhileTheNameServiceDoesNotAnswer(t *testing.T) {
	var blocked atomic.Bool
	release := make(chan struct{})
	defer close(release)
	r := New(func(uid uint32) (Identity, error) {
		if blocked.Load() {
			<-release
		}
		return Identity{UID: uid, Login: "alice"}, nil
	})
	r.positiveTTL = 10 * time.Millisecond
	r.timeout = 50 * time.Millisecond
	_, err := r.Resolve(context.Background(), 1001)
	require.NoError(t, err)
	blocked.Store(true)
	time.Sleep(20 * time.Millisecond)
	id, err := r.Resolve(context.Background(), 1001)
	require.NoError(t, err, "the identity last seen")
	assert.Equal(t, "alice", id.Login)
	_, err = r.Resolve(context.Background(), 1002)
	assert.ErrorIs(t, err, ErrTimeout, "an identity never seen")

	r.mu.Lock()
	r.known[1001] = entry{id: r.known[1001].id, expires: time.Now()}
	r.mu.Unlock()
	_, err = r.Resolve(context.Background(), 1001)
	assert.ErrorIs(t, err, ErrTimeout, "an identity seen too long ago")
}

// A name service answering an identity is gone has the last word: the
// identity last seen is not kept for it.
func TestResolveForgetsAnIdentityTheNameServiceRemoved(t *testing.T) {
	var gone atomic.Bool
	r := New(func(uid uint32) (Identity, error) {
		if gone.Load() {
			return Identity{}, errors.New("unknown uid")
		}
		return Identity{UID: uid, Login: "alice"}, nil
	})
	r.positiveTTL = 10 * time.Millisecond
	_, err := r.Resolve(context.Background(), 1001)
	require.NoError(t, err)
	gone.Store(true)
	time.Sleep(20 * time.Millisecond)
	_, err = r.Resolve(context.Background(), 1001)
	assert.Error(t, err)
	r.mu.Lock()
	_, ok := r.known[1001]
	r.mu.Unlock()
	assert.False(t, ok)
}

func TestSystemLookup(t *testing.T) {
	id, err := SystemLookup(uint32(os.Getuid()))
	require.NoError(t, err)
	assert.NotEmpty(t, id.Login)
	assert.NotEmpty(t, id.Groups)
}
