package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shaj13/go-guardian/v2/auth"

	"github.com/opensvc/om3/daemon/api"
)

// PostAuthRefresh create a new token for the refresh token user
func (a *DaemonAPI) PostAuthRefresh(ctx echo.Context, params api.PostAuthRefreshParams) error {
	if !a.canCreateAccessToken(ctx) {
		return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "not allowed to create token")
	}

	// check params
	duration, err := a.accessTokenDuration(params.AccessDuration)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Invalid access duration: %s", err)
	} else if err := validateRole(params.Role); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Invalid role: %s", err)
	}

	username := ctx.Get("user").(auth.Info).GetUserName()
	if d, err := a.createAccessToken(ctx, username, duration, params.Role, params.Scope); err != nil {
		log := LogHandler(ctx, "PostAuthRefresh")
		if errors.Is(err, errBadRequest) {
			log.Tracef("invalid parameters: %s", err)
			return JSONProblemf(ctx, http.StatusBadRequest, "Create token Invalid parameters", "%s", err)
		} else if errors.Is(err, errForbidden) {
			log.Infof("forbidden: %s", err)
			return JSONProblemf(ctx, http.StatusForbidden, "Create token Forbidden", "%s", err)
		}
		log.Warnf("create token: %v", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Create token", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, d)
	}
}
