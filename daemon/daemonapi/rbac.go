package daemonapi

import (
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/rbac"
)

func hasAnyRole(ctx echo.Context, roles ...rbac.Role) bool {
	user := User(ctx)
	userGrants := rbac.NewGrants(user.GetExtensions()["grant"]...)
	return rbac.MatchRoles(userGrants, roles...)
}

func assertRoleRoot(ctx echo.Context) error {
	neededRoles := []rbac.Role{rbac.RoleRoot}
	if !hasAnyRole(ctx, neededRoles...) {
		return JSONForbiddenMissingRole(ctx, neededRoles...)
	}
	return nil
}
