package daemonauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/peercred"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/uxid"
)

type (
	// UXUserGranter is the interface of the users bound to the local
	// accounts connected to the user socket.
	UXUserGranter interface {
		UsrFromUXUser(login string, uid uint32) (string, []string, error)
	}

	// uxUserStrategy authenticates the local accounts connected to the
	// user socket, by the credentials the kernel recorded at connect.
	uxUserStrategy struct {
		users    UXUserGranter
		resolver *uxid.Resolver
		groups   func() map[string][]string
		log      *plog.Logger
	}
)

// uxResolver is shared by the strategies the auth configuration changes
// make again: its lookups and its cache outlive them.
var uxResolver = uxid.New(nil)

// Authenticate grants a process connected to the user socket the grants of
// its unix groups, as the [uxgroup#<group>] sections say them, and of the
// user whose uxuser names its login or uid.
//
// The identity is the one the kernel recorded at connect, which the process
// can not choose. Root is not one: it uses the root socket, and a rootful
// container handed the user socket would otherwise be root. A container is
// not judged by its uid either, which its own user namespace may map to any
// local account.
func (t *uxUserStrategy) Authenticate(ctx context.Context, _ *http.Request) (*Info, error) {
	if daemonctx.ListenAddr(ctx) != daemonenv.HTTPUserUnixFile() {
		return nil, fmt.Errorf("strategies/uxuser: not a client of the user socket")
	}
	peer := daemonctx.Peer(ctx)
	if peer == nil {
		return nil, fmt.Errorf("strategies/uxuser: the peer of the connection is unknown")
	}
	switch {
	case peer.UID == 0:
		return nil, fmt.Errorf("strategies/uxuser: root uses the root socket")
	case peer.ForeignMountNS:
		return nil, fmt.Errorf("strategies/uxuser: a container is not identified by its uid")
	}
	key := cacheKey(StrategyUXUser, strconv.FormatUint(uint64(peer.UID), 10))
	if info, ok := authCache.get(key); ok {
		return info, nil
	}
	info, err := t.authenticate(ctx, peer)
	if err != nil {
		return nil, err
	}
	authCache.set(key, info, cacheTTL)
	return info, nil
}

func (t *uxUserStrategy) authenticate(ctx context.Context, peer *peercred.T) (*Info, error) {
	// A user naming the uid is found without the name service: it keeps
	// its grants while a directory server does not answer.
	id, nsErr := t.resolver.Resolve(ctx, peer.UID)
	usrName, usrGrants, err := t.users.UsrFromUXUser(id.Login, peer.UID)
	if err != nil {
		return nil, fmt.Errorf("strategies/uxuser: uid %d: %w", peer.UID, err)
	}
	if nsErr != nil && usrName == "" {
		if errors.Is(nsErr, uxid.ErrBusy) || errors.Is(nsErr, uxid.ErrTimeout) {
			t.log.Warnf("uid %d: %s", peer.UID, nsErr)
		}
		return nil, fmt.Errorf("strategies/uxuser: uid %d: %w", peer.UID, nsErr)
	}
	grants := slices.Clone(usrGrants)
	granting := make([]string, 0)
	groups := t.groups()
	for _, group := range id.Groups {
		l, ok := groups[group]
		if !ok {
			continue
		}
		granting = append(granting, group)
		for _, g := range l {
			if !slices.Contains(grants, g) {
				grants = append(grants, g)
			}
		}
	}
	username := usrName
	if username == "" {
		username = id.Login
	}
	t.log.Infof("uid=%d login=%s usr=%s groups=%s grants=%s", peer.UID, id.Login, usrName, strings.Join(granting, ","), strings.Join(grants, " "))
	return &Info{
		Username: username,
		Strategy: StrategyUXUser,
		Issuer:   hostname.Hostname(),
		Grants:   grants,
	}, nil
}

func initUXUser(ctx context.Context, i interface{}) (string, Strategy, error) {
	name := "ux user"
	users, ok := i.(UXUserGranter)
	if !ok {
		return name, nil, fmt.Errorf("missing UXUserGranter interface")
	}
	uxResolver.Flush()
	return name, &uxUserStrategy{
		users:    users,
		resolver: uxResolver,
		groups:   func() map[string][]string { return cluster.ConfigData.Get().UXGroups },
		log:      plog.NewDefaultLogger().WithPrefix("daemon: auth: uxuser: ").Attr("pkg", "daemon/auth"),
	}, nil
}
