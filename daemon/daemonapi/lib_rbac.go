package daemonapi

import (
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/daemon/rbac"
)

// assertGuest asserts that the authenticated user has is either granted the "guest", "operator" or "admin" role on the namespace or is granted the "root" role.
func assertGuest(ctx echo.Context, namespace string) (bool, error) {
	return assertGrant(ctx, rbac.NewGrant(rbac.RoleGuest, namespace), rbac.NewGrant(rbac.RoleOperator, namespace), rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantJoin, rbac.GrantRoot)
}

// assertOperator asserts that the authenticated user has is either granted the "operator" or "admin" role on the namespace or is granted the "root" role.
func assertOperator(ctx echo.Context, namespace string) (bool, error) {
	return assertGrant(ctx, rbac.NewGrant(rbac.RoleOperator, namespace), rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot)
}

// assertAdmin asserts that the authenticated user has is either granted the "admin" role on the namespace or is granted the "root" role.
func assertAdmin(ctx echo.Context, namespace string) (bool, error) {
	return assertGrant(ctx, rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot)
}

// assertRoot asserts that the authenticated user has is granted the "root" role.
func assertRoot(ctx echo.Context) (bool, error) {
	return assertGrant(ctx, rbac.GrantRoot)
}

func assertStrategy(ctx echo.Context, expected string) (bool, error) {
	if strategy := strategyFromContext(ctx); strategy != expected {
		return false, JSONForbiddenStrategy(ctx, strategy, expected)
	}
	return true, nil
}

func assertGrant(ctx echo.Context, grants ...rbac.Grant) (bool, error) {
	if !grantsFromContext(ctx).HasGrant(grants...) {
		return false, JSONForbiddenMissingGrant(ctx, grants...)
	}
	return true, nil
}

func assertRole(ctx echo.Context, roles ...rbac.Role) (bool, error) {
	if !grantsFromContext(ctx).HasRole(roles...) {
		return false, JSONForbiddenMissingRole(ctx, roles...)
	}
	return true, nil
}

func keyopStringRbac(op string) error {
	kop := keyop.Parse(op)
	if kop == nil {
		return fmt.Errorf("invalid op: %s", op)
	}
	return keyopRbac(*kop)
}

func keyopRbac(op keyop.T) error {
	if strings.HasSuffix(op.Key.Option, "_trigger") {
		return fmt.Errorf("triggers requires the root grant")
	}
	drvGroup := strings.Split(op.Key.Section, "#")[0]
	switch drvGroup {
	case "app", "task":
		switch op.Key.Option {
		case "script", "start", "stop", "check", "info":
			return fmt.Errorf("app and task commands require the root grant")
		}
	case "container":
		switch op.Key.Option {
		case "volume_mounts":
			for _, e := range strings.Fields(op.Value) {
				if strings.HasPrefix(e, "_") || strings.Contains(e, "/../") || strings.HasPrefix(e, "../") || strings.HasSuffix(e, "../") {
					return fmt.Errorf("host path mounts in container require the root grant")
				}
			}
		}
	}
	return nil
}
