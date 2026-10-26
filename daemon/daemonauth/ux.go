package daemonauth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

type (
	uxStrategy struct {
		getter ListenAddresser
	}

	// ListenAddresser is the interface for ListenAddr method for ux auth.
	ListenAddresser interface {
		ListenAddr(context.Context) string
	}
)

// Authenticate grants root to a request of the root unix socket, whose
// directory only root reaches. A request of any other listener, as the user
// socket a group reaches, is not one it vouches for.
func (t uxStrategy) Authenticate(ctx context.Context, _ *http.Request) (*Info, error) {
	addr := t.getter.ListenAddr(ctx)
	if addr != daemonenv.HTTPUnixFile() {
		return nil, fmt.Errorf("strategies/ux: not a client of the root unix socket (%s)", addr)
	}
	return &Info{
		Username: "root",
		Strategy: StrategyUX,
		Grants:   []string{rbac.GrantRoot.String()},
	}, nil
}

func initUX(_ context.Context, i interface{}) (string, Strategy, error) {
	name := "ux auth"
	fn, ok := i.(ListenAddresser)
	if !ok {
		return name, nil, fmt.Errorf("missing ListenAddresser interface")
	}
	return name, &uxStrategy{getter: fn}, nil
}
