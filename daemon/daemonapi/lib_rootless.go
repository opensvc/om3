package daemonapi

import (
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rootless"
)

// warnSharedRootlessAccounts says, when a namespace configuration was
// written, the accounts it allows that other namespaces allow too.
//
// Two namespaces running containers as the same account reach each other's
// containers, images and files. The squatter may decide so, and is not
// refused, but it is said where the write is logged, and by the status of
// every container running as the account.
func warnSharedRootlessAccounts(ctx echo.Context, p naming.Path) {
	if p.Kind != naming.KindNscfg {
		return
	}
	allowed, err := rootless.Load(p.Namespace)
	if err != nil {
		return
	}
	shared, err := allowed.SharedWith()
	if err != nil || len(shared) == 0 {
		return
	}
	LogHandler(ctx, "config").Warnf("the %s namespace shares rootless accounts, so each namespace reaches the containers of the other: %s", p.Namespace, rootless.DescribeShared(shared))
}
