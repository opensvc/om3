package daemonapi

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shaj13/go-guardian/v2/auth"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonauth"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/util/converters"
)

// PostAuthToken create a new token for a user
//
// When role parameter exists a new user is created with grants from role and
// extra claims may be added to token
func (a *DaemonApi) PostAuthToken(ctx echo.Context, params api.PostAuthTokenParams) error {
	if err := assertRoleRoot(ctx); err != nil {
		return err
	}
	var (
		// duration define the default token duration
		duration = time.Minute * 10

		// duration define the maximum token duration
		durationMax = time.Hour * 24

		xClaims = make(daemonauth.Claims)
	)
	log := LogHandler(ctx, "PostAuthToken")
	if params.Duration != nil {
		if v, err := converters.Duration.Convert(*params.Duration); err != nil {
			log.Info().Err(err).Msgf("invalid duration: %s", *params.Duration)
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Invalid duration: %s", *params.Duration)
		} else {
			duration = *v.(*time.Duration)
			if duration > durationMax {
				duration = durationMax
			}
		}
	}
	user := ctx.Get("user").(auth.Info)
	username := user.GetUserName()
	// TODO verify if user is allowed to create token => 403 Forbidden
	if params.Role != nil {
		var err error
		user, xClaims, err = userXClaims(params, user)
		if err != nil {
			log.Error().Err(err).Msg("userXClaims")
			return JSONProblemf(ctx, http.StatusServiceUnavailable, "Invalid user claims", "user name: %s", username)
		}
	}

	tk, expireAt, err := daemonauth.CreateUserToken(user, duration, xClaims)
	if err != nil {
		switch err {
		case daemonauth.NotImplementedError:
			log.Warn().Err(err).Send()
			return JSONProblemf(ctx, http.StatusNotImplemented, err.Error(), "")
		default:
			log.Error().Err(err).Msg("can't create token")
			return JSONProblemf(ctx, http.StatusInternalServerError, "Unexpected error", "%s", err)
		}
	}
	return ctx.JSON(http.StatusOK, api.AuthToken{
		ExpiredAt: expireAt,
		Token:     tk,
	})
}

// userXClaims returns new user and Claims from p and current user
func userXClaims(p api.PostAuthTokenParams, srcInfo auth.Info) (info auth.Info, xClaims daemonauth.Claims, err error) {
	xClaims = make(daemonauth.Claims)
	grants := daemonauth.Grants{}
	roleDone := make(map[api.Role]bool)
	for _, r := range *p.Role {
		if _, ok := roleDone[r]; ok {
			continue
		}
		role := daemonauth.Role(r)
		switch role {
		case daemonauth.RoleJoin:
			var b []byte
			filename := daemonenv.CertChainFile()
			b, err = os.ReadFile(filename)
			if err != nil {
				return
			}
			xClaims["ca"] = string(b)
		case daemonauth.RoleAdmin:
		case daemonauth.RoleBlacklistAdmin:
		case daemonauth.RoleGuest:
		case daemonauth.RoleHeartbeat:
		case daemonauth.RoleLeave:
		case daemonauth.RoleRoot:
		case daemonauth.RoleSquatter:
		case daemonauth.RoleUndef:
		default:
			err = fmt.Errorf("%w: unexpected role %s", echo.ErrBadRequest, role)
			return
		}
		grants = append(grants, daemonauth.Grant(r))
		roleDone[r] = true
	}
	userName := srcInfo.GetUserName()
	info = auth.NewUserInfo(userName, userName, nil, grants.Extensions())
	return
}
