package daemonauth

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/util/peercred"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/uxid"
)

type stubUsers map[string]struct {
	name   string
	grants []string
}

func (t stubUsers) UsrFromUXUser(login string, uid uint32) (string, []string, error) {
	for k, u := range t {
		if k == login || k == strconv.FormatUint(uint64(uid), 10) {
			return u.name, u.grants, nil
		}
	}
	return "", nil, nil
}

func newTestUXUserStrategy(users stubUsers, lookup uxid.LookupFunc) *uxUserStrategy {
	return &uxUserStrategy{
		users:    users,
		resolver: uxid.New(lookup),
		groups: func() map[string][]string {
			return map[string][]string{"osvc-ns1": {"operator:ns1"}, "osvc-ro": {"guest:*"}}
		},
		log: plog.NewDefaultLogger(),
	}
}

func uxUserContext(peer *peercred.T) context.Context {
	ctx := daemonctx.WithListenAddr(context.Background(), daemonenv.HTTPUserUnixFile())
	return daemonctx.WithPeer(ctx, peer)
}

func TestUXUserStrategy(t *testing.T) {
	authCache = newCache()
	alice := func(uid uint32) (uxid.Identity, error) {
		return uxid.Identity{UID: uid, Login: "alice", Groups: []string{"alice", "osvc-ns1", "osvc-ro"}}, nil
	}
	down := func(uid uint32) (uxid.Identity, error) {
		return uxid.Identity{}, errors.New("the directory does not answer")
	}

	t.Run("the grants of the groups", func(t *testing.T) {
		authCache = newCache()
		s := newTestUXUserStrategy(stubUsers{}, alice)
		info, err := s.Authenticate(uxUserContext(&peercred.T{UID: 1001}), nil)
		require.NoError(t, err)
		assert.Equal(t, "alice", info.Username)
		assert.Equal(t, StrategyUXUser, info.Strategy)
		assert.ElementsMatch(t, []string{"operator:ns1", "guest:*"}, info.Grants)
	})
	t.Run("the grants of the user bound to the login, with the groups'", func(t *testing.T) {
		authCache = newCache()
		s := newTestUXUserStrategy(stubUsers{"alice": {"u-alice", []string{"admin:ns2", "guest:*"}}}, alice)
		info, err := s.Authenticate(uxUserContext(&peercred.T{UID: 1001}), nil)
		require.NoError(t, err)
		assert.Equal(t, "u-alice", info.Username)
		assert.ElementsMatch(t, []string{"admin:ns2", "guest:*", "operator:ns1"}, info.Grants)
	})
	t.Run("a user bound to the uid keeps its grants with the name service down", func(t *testing.T) {
		authCache = newCache()
		s := newTestUXUserStrategy(stubUsers{"1001": {"u-1001", []string{"admin:ns2"}}}, down)
		info, err := s.Authenticate(uxUserContext(&peercred.T{UID: 1001}), nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"admin:ns2"}, info.Grants)
	})
	t.Run("an account the name service can not say is refused", func(t *testing.T) {
		authCache = newCache()
		s := newTestUXUserStrategy(stubUsers{}, down)
		_, err := s.Authenticate(uxUserContext(&peercred.T{UID: 1001}), nil)
		assert.ErrorContains(t, err, "does not answer")
	})
	t.Run("root is not root here", func(t *testing.T) {
		s := newTestUXUserStrategy(stubUsers{}, alice)
		_, err := s.Authenticate(uxUserContext(&peercred.T{UID: 0}), nil)
		assert.ErrorContains(t, err, "root uses the root socket")
	})
	t.Run("a container is not judged by its uid", func(t *testing.T) {
		s := newTestUXUserStrategy(stubUsers{}, alice)
		_, err := s.Authenticate(uxUserContext(&peercred.T{UID: 1001, ForeignMountNS: true}), nil)
		assert.ErrorContains(t, err, "container")
	})
	t.Run("another listener is not the user socket", func(t *testing.T) {
		s := newTestUXUserStrategy(stubUsers{}, alice)
		ctx := daemonctx.WithPeer(daemonctx.WithListenAddr(context.Background(), daemonenv.HTTPUnixFile()), &peercred.T{UID: 1001})
		_, err := s.Authenticate(ctx, nil)
		assert.Error(t, err)
	})
}
