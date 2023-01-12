package daemonapi

import (
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/shaj13/go-guardian/v2/auth"

	"opensvc.com/opensvc/daemon/daemonauth"
)

func (a *DaemonApi) PostAuthToken(w http.ResponseWriter, r *http.Request) {
	duration := time.Minute * 10
	user := auth.User(r)
	// TODO verify if user is allowed to create token => 403 Forbidden
	tk, expireAt, err := daemonauth.CreateUserToken(user, duration, nil)
	if err != nil {
		log := getLogger(r, "PostAuthToken")
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
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ResponsePostAuthToken{
		Token:         tk,
		TokenExpireAt: expireAt,
	})
}
