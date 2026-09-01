package daemonauth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/basic"

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
)

func initBasicUser(_ context.Context, i any) (string, auth.Strategy, error) {
	name := "basicauth user"
	userDB, ok := i.(UserAndPasswordGranter)
	if !ok {
		return name, nil, fmt.Errorf("UserAndPasswordGranter interface is not implemented")
	}
	validateUser := func(_ context.Context, _ *http.Request, userName string, password string) (auth.Info, error) {
		grants, err := userDB.GrantsFromUsernameAndPassword(userName, password)
		if err != nil {
			return nil, fmt.Errorf("invalid user %s: %w", userName, err)
		}
		return auth.NewUserInfo(userName, "", nil, *authenticatedExtensions(StrategyUser, hostname.Hostname(), grants...)), nil
	}
	return name, basic.NewCached(validateUser, cache), nil
}
