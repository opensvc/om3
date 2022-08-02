package daemonauth

import (
	"net/http"
	"time"

	"github.com/shaj13/go-guardian/v2/auth"
)

type TokenResponse struct {
	Token         string    `json:"token"`
	TokenExpireAt time.Time `json:"token_expire_at"`
}

//
// GetNodes     godoc
// @Summary      Get a node authentication token
// @Description  Get an authentication token from a node's credentials submitted with basic login.
// @Security     BasicAuth
// @Security     BearerAuth
// @Tags         auth
// @Produce      json
// @Success      200  {object}  TokenResponse
// @Failure      403  {string}  string
// @Failure      500  {string}  string  "Internal Server Error"
// @Router       /auth/node/token  [get]
//
func GetNodeToken(w http.ResponseWriter, r *http.Request) {
	exp := time.Minute * 10
	user := auth.User(r)
	tokenExpireAt := time.Now().Add(exp)
	claims := map[string]interface{}{
		"exp":        tokenExpireAt.Unix(),
		"authorized": true,
		"grant":      user.GetExtensions()["grant"],
	}
	_, token, err := TokenAuth.Encode(claims)
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}
	AppendToken(token, user)

	jsonEncode(w, TokenResponse{
		TokenExpireAt: tokenExpireAt,
		Token:         token,
	})
}
