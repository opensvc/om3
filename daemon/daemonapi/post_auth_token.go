package daemonapi

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shaj13/go-guardian/v2/auth"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/util/converters"
)

// PostAuthToken create a new token for a user
//
// When role parameter exists a new user is created with grants from role and
// extra claims may be added to token
func (a *DaemonAPI) PostAuthToken(ctx echo.Context, params api.PostAuthTokenParams) error {
	if v, err := assertRole(ctx, rbac.RoleRoot); err != nil {
		return err
	} else if !v {
		return nil
	}
	var (
		// duration define the default token duration
		duration = time.Minute * 10

		// duration define the maximum token duration
		durationMax = time.Hour * 24

		xClaims = make(map[string]interface{})
	)
	name := "PostAuthToken"
	log := LogHandler(ctx, name)
	if params.Duration != nil {
		if v, err := converters.Duration.Convert(*params.Duration); err != nil {
			log.Infof("%s: invalid duration: %s: %s", name, *params.Duration, err)
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
			log.Errorf("%s: userXClaims: %s", name, err)
			return JSONProblemf(ctx, http.StatusServiceUnavailable, "Invalid user claims", "user name: %s", username)
		}
	}

	xClaims["iss"] = a.localhost
	tk, expireAt, err := a.JWTcreator.CreateUserToken(user, duration, xClaims)
	if err != nil {
		log.Errorf("%s: can't create token: %s", name, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Unexpected error", "%s", err)
	} else if tk == "" {
		err := fmt.Errorf("create token error: jwt auth is not enabled")
		log.Warnf("%s: %s", name, err)
		return JSONProblemf(ctx, http.StatusNotImplemented, err.Error(), "")
	}
	return ctx.JSON(http.StatusOK, api.AuthToken{
		ExpiredAt: expireAt,
		Token:     tk,
	})
}

// userXClaims returns new user and Claims from p and current user
func userXClaims(p api.PostAuthTokenParams, srcInfo auth.Info) (info auth.Info, xClaims map[string]interface{}, err error) {
	xClaims = make(map[string]interface{})
	extensions := auth.Extensions{"grant": []string{}}
	roleDone := make(map[api.Role]bool)
	for _, r := range *p.Role {
		if _, ok := roleDone[r]; ok {
			continue
		}
		role := rbac.Role(r)
		switch role {
		case rbac.RoleJoin:
			var b []byte
			filename := daemonenv.CertChainFile()
			b, err = os.ReadFile(filename)
			if err != nil {
				return
			}
			xClaims["ca"] = string(b)
		case rbac.RoleAdmin:
		case rbac.RoleBlacklistAdmin:
		case rbac.RoleGuest:
		case rbac.RoleHeartbeat:
		case rbac.RoleLeave:
		case rbac.RoleRoot:
		case rbac.RoleSquatter:
		case rbac.RoleUndef:
			continue
		default:
			err = fmt.Errorf("%w: unexpected role %s", echo.ErrBadRequest, role)
			return
		}
		extensions.Add("grant", string(r))
		roleDone[r] = true
	}
	userName := srcInfo.GetUserName()
	info = auth.NewUserInfo(userName, userName, nil, extensions)
	return
}
