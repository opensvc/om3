package daemonapi

import (
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/daemonauth"
)

func hasAnyRole(ctx echo.Context, role ...daemonauth.Role) bool {
	return Grants(User(ctx)).HasAnyRole(role...)
}

func assertRoleRoot(ctx echo.Context) error {
	neededRoles := []daemonauth.Role{daemonauth.RoleRoot}
	if !hasAnyRole(ctx, neededRoles...) {
		return JSONForbiddenMissingRole(ctx, neededRoles...)
	}
	return nil
}
