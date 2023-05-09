package daemonapi

import (
	"net/http"
	"os"
	"time"

	"github.com/goccy/go-json"
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
func (a *DaemonApi) PostAuthToken(w http.ResponseWriter, r *http.Request, params api.PostAuthTokenParams) {
	var (
		// duration define the default token duration
		duration = time.Minute * 10

		// duration define the maximum token duration
		durationMax = time.Hour * 24

		xClaims = make(daemonauth.Claims)
	)
	log := getLogger(r, "PostAuthToken")
	if params.Duration != nil {
		if v, err := converters.Duration.Convert(*params.Duration); err != nil {
			log.Info().Err(err).Msgf("invalid duration: %s", *params.Duration)
			sendError(w, http.StatusBadRequest, "invalid duration")
			return
		} else {
			duration = *v.(*time.Duration)
			if duration > durationMax {
				duration = durationMax
			}
		}
	}
	user := auth.User(r)
	// TODO verify if user is allowed to create token => 403 Forbidden
	if params.Role != nil {
		grants := daemonauth.UserGrants(r)
		if !grants.HasRoot() {
			log.Info().Msg("not allowed, need grant root")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var err error
		user, xClaims, err = userXClaims(params, user)
		if err != nil {
			log.Error().Err(err).Msg("userXClaims")
			sendError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
	}

	tk, expireAt, err := daemonauth.CreateUserToken(user, duration, xClaims)
	if err != nil {
		switch err {
		case daemonauth.NotImplementedError:
			log.Warn().Err(err).Msg("")
			sendError(w, http.StatusNotImplemented, err.Error())
		default:
			log.Error().Err(err).Msg("can't create token")
			sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(api.ResponsePostAuthToken{
		Token:         tk,
		TokenExpireAt: expireAt,
	})
	if err != nil {
		log.Error().Err(err).Msg("json encode")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
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
			grants = append(grants, daemonauth.Grant(r))
		}
		roleDone[r] = true
	}
	userName := srcInfo.GetUserName()
	info = auth.NewUserInfo(userName, userName, nil, grants.Extensions())
	return
}
