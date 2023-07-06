package daemonauth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/basic"
)

type (
	// UserGranter is the interface for UserGrants method for user basic auth.
	UserGranter interface {
		UserGrants(username, password string) ([]string, error)
	}

	// NodeAuthenticater is the interface for AuthenticateNode method for node basic auth.
	NodeAuthenticater interface {
		AuthenticateNode(nodename, password string) error
	}
)

func initBasicUser(i any) (string, auth.Strategy, error) {
	name := "basicauth user"
	userDB, ok := i.(UserGranter)
	if !ok {
		return name, nil, fmt.Errorf("UserGranter not implemented")
	}
	validateUser := func(_ context.Context, _ *http.Request, userName string, password string) (auth.Info, error) {
		grants, err := userDB.UserGrants(userName, password)
		if err != nil {
			return nil, fmt.Errorf("invalid user %s: %w", userName, err)
		}
		return auth.NewUserInfo(userName, "", nil, *authenticatedExtensions("user", grants...)), nil
	}
	return name, basic.NewCached(validateUser, cache), nil
}

func initBasicNode(i interface{}) (string, auth.Strategy, error) {
	name := "basicauth node"
	n, ok := i.(NodeAuthenticater)
	if !ok {
		return name, nil, fmt.Errorf("missing node authenticater")
	}
	validate := func(_ context.Context, _ *http.Request, userName string, password string) (auth.Info, error) {
		if err := n.AuthenticateNode(userName, password); err != nil {
			return nil, fmt.Errorf("invalid nodename %s: %w", userName, err)
		}
		extensions := authenticatedExtensions("node", "root")
		info := auth.NewUserInfo("node-"+userName, "", nil, *extensions)
		return info, nil
	}
	return name, basic.NewCached(validate, cache), nil
}
