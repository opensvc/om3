package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) Getwhoami(ctx echo.Context) error {
	pts := func(s string) *string { return &s }
	user := userFromContext(ctx)
	extensions := user.GetExtensions()
	grants := grantsFromContext(ctx)
	data := api.UserIdentity{
		Auth:      pts(extensions.Get("strategy")),
		Grant:     map[string][]string{},
		Name:      user.GetUserName(),
		Namespace: "system",
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
