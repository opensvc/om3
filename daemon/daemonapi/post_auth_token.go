package daemonapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shaj13/go-guardian/v2/auth"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

// PostAuthToken create a new token for a user
//
// When role parameter exists a new user is created with grants from role and
// extra claims may be added to token
func (a *DaemonAPI) PostAuthToken(ctx echo.Context, params api.PostAuthTokenParams) error {
	if !a.canCreateAccessToken(ctx) {
		return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "not allowed to create token")
	}
	needRefresh := params.Refresh != nil && *params.Refresh
	if needRefresh {
		if !a.canCreateRefreshToken(ctx) {
			return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "not allowed to create refresh token")
		}
	}
	var (
		data = api.AuthToken{}

		log = LogHandler(ctx, "PostAuthToken")

		refreshDuration time.Duration
	)

	// check params
	accessDuration, err := a.accessTokenDuration(params.AccessDuration)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Invalid access duration: %s", err)
	}
	if needRefresh {
		if refreshDuration, err = a.refreshTokenDuration(params.RefreshDuration); err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Invalid refresh duration: %s", err)
		}
	}
	if err := validateRole(params.Role); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Invalid role: %s", err)
	}

	username := ctx.Get("user").(auth.Info).GetUserName()

	if params.Subject != nil && *params.Subject != "" {
		if v, err := assertRole(ctx, rbac.RoleRoot); err != nil {
			return err
		} else if !v {
			log.Errorf("user grants for subject '%s' refused: need %s role", *params.Subject, rbac.RoleRoot)
			return nil
		}
		username = *params.Subject
	}

	if d, err := a.createAccessToken(ctx, username, accessDuration, params.Role, params.Scope); err != nil {
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
		data.AccessToken = d.AccessToken
		data.AccessExpiredAt = d.AccessExpiredAt
	}

	if needRefresh {
		if rk, exp, err := a.createToken(username, "refresh", refreshDuration, nil); err != nil {
			log.Errorf("create refresh token: %s", err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "Unexpected error", "%s", err)
		} else {
			data.RefreshToken = &rk
			data.RefreshExpiredAt = &exp
		}
	}

	return ctx.JSON(http.StatusOK, data)
}
