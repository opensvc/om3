package daemonauth

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/shaj13/go-guardian/v2/auth"
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

func (t uxStrategy) Authenticate(ctx context.Context, _ *http.Request) (auth.Info, error) {
	addr := t.getter.ListenAddr(ctx)
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return nil, fmt.Errorf("strategies/ux: is a inet address family client (%s)", addr) // How to continue ?
	}
	info := auth.NewUserInfo("root", "", nil, *authenticatedExtensions("ux", "", "root"))
	return info, nil
}

func initUX(i interface{}) (string, auth.Strategy, error) {
	name := "ux auth"
	fn, ok := i.(ListenAddresser)
	if !ok {
		return name, nil, fmt.Errorf("missing ListenAddresser interface")
	}
	return name, &uxStrategy{getter: fn}, nil
}
