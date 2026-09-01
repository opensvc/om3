package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetAuthWhoAmI(ctx echo.Context) error {
	pts := func(s string) *string { return &s }
	user := userFromContext(ctx)
	grants := grantsFromContext(ctx)
	data := api.UserIdentity{
		Auth:      pts(user.Strategy),
		Grant:     map[string][]string{},
		Name:      user.Username,
		Namespace: naming.NsSys,
		RawGrant:  grants.String(),
	}
	for _, grant := range grants {
		role, scope := grant.Split()
		switch scope {
		case "":
			if _, ok := data.Grant[role]; !ok {
				data.Grant[role] = nil
			}
		default:
			_, ok := data.Grant[role]
			switch ok {
			case false:
				data.Grant[role] = []string{scope}
			default:
				data.Grant[role] = append(data.Grant[role], scope)
			}
		}
	}
	return ctx.JSON(http.StatusOK, data)
}
