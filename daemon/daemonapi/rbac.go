package daemonapi

import (
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/rbac"
)

func hasAnyRole(ctx echo.Context, roles ...rbac.Role) bool {
	userGrants := grantsFromContext(ctx)
	return userGrants.HasAnyRole(roles...)
}

func assertRoleRoot(ctx echo.Context) error {
	neededRoles := []rbac.Role{rbac.RoleRoot}
	if !hasAnyRole(ctx, neededRoles...) {
		return JSONForbiddenMissingRole(ctx, neededRoles...)
	}
	return nil
}
