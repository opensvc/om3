package daemonapi

import (
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/daemonauth"
)

// canCreateAccessToken determines whether an access token can be created based
// on the token type (refresh token) or authentication strategy (UX or User).
func (a *DaemonAPI) canCreateAccessToken(ctx echo.Context) bool {
	if s, ok := ctx.Get("token_use").(string); ok && s == "refresh" {
		return true
	}
	strategy := strategyFromContext(ctx)
	switch strategy {
	case daemonauth.StrategyUX:
	case daemonauth.StrategyUser:
	default:
		return false
	}
	return true
}

// canCreateRefreshToken determines if a refresh token can be created based
// on the authentication strategy from the context: It needs StrategyUX or
// StrategyUser.
func (a *DaemonAPI) canCreateRefreshToken(ctx echo.Context) bool {
	strategy := strategyFromContext(ctx)
	switch strategy {
	case daemonauth.StrategyUX:
	case daemonauth.StrategyUser:
	default:
		return false
	}
	return true
}
