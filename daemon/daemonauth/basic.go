package daemonauth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	// UserAndPasswordGranter is the interface for UserGrants method for user basic auth.
	UserAndPasswordGranter interface {
		GrantsFromUsernameAndPassword(username, password string) ([]string, error)
	}

	// UserGranter is the interface for UserGrants method for user basic auth.
	UserGranter interface {
		GrantsFromUsername(username string) ([]string, error)
	}

	basicUserStrategy struct {
		userDB UserAndPasswordGranter
	}
)

// Authenticate reads the credentials of a usr object from the request's
// basic auth header.
//
// The answer is cached for a few seconds because verifying it reads and
// decrypts the usr object from disk, and a client polling the api
// presents the same password on every request.
func (t *basicUserStrategy) Authenticate(_ context.Context, r *http.Request) (*Info, error) {
	username, password, ok := r.BasicAuth()
	if !ok {
		return nil, fmt.Errorf("strategies/basic: request has no basic auth credentials")
	}
	key := cacheKey(StrategyUser, username, password)
	if info, ok := authCache.get(key); ok {
		return info, nil
	}
	grants, err := t.userDB.GrantsFromUsernameAndPassword(username, password)
	if err != nil {
		return nil, fmt.Errorf("strategies/basic: invalid user %s: %w", username, err)
	}
	info := &Info{
		Username: username,
		Strategy: StrategyUser,
		Issuer:   hostname.Hostname(),
		Grants:   grants,
	}
	authCache.set(key, info, cacheTTL)
	return info, nil
}

func initBasicUser(_ context.Context, i any) (string, Strategy, error) {
	name := "basicauth user"
	userDB, ok := i.(UserAndPasswordGranter)
	if !ok {
		return name, nil, fmt.Errorf("UserAndPasswordGranter interface is not implemented")
	}
	return name, &basicUserStrategy{userDB: userDB}, nil
}
