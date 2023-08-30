package daemonapi

import (
	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/daemon/rbac"
)

func assertRole(ctx echo.Context, roles ...rbac.Role) (bool, error) {
	if !grantsFromContext(ctx).HasRole(roles...) {
		if err := JSONForbiddenMissingRole(ctx, roles...); err != nil {
			return false, err
		} else {
			return false, nil
		}
	}
	return true, nil
}
