package daemonauth

import (
	"context"
	"fmt"
	"net"
	"net/http"

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

func (t uxStrategy) Authenticate(ctx context.Context, _ *http.Request) (*Info, error) {
	addr := t.getter.ListenAddr(ctx)
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return nil, fmt.Errorf("strategies/ux: is a inet address family client (%s)", addr) // How to continue ?
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
