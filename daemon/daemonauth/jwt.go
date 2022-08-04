package daemonauth

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/jwtauth/v5"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/token"
	"golang.org/x/crypto/ssh"
	"opensvc.com/opensvc/daemon/daemonenv"
)

type (
	// TokenResponse is the struct returned as response to GET /auth/token
	TokenResponse struct {
		Token         string    `json:"token"`
		TokenExpireAt time.Time `json:"token_expire_at"`
	}
)

var (
	tokenStrategy    auth.Strategy
	TokenAuth        *jwtauth.JWTAuth
	verifyBytes      []byte
	verifyKey        *rsa.PublicKey
	signKey          *rsa.PrivateKey
	jwtSignKeyFile   string
	jwtVerifyKeyFile string
)

func jsonEncode(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func initToken() auth.Strategy {
	log.Logger.Info().Msg("init token auth strategy")
	if err := initJWT(); err != nil {
		return nil
	}
	tokenStrategy = token.New(token.NoOpAuthenticate, cache)
	return tokenStrategy
}

func initJWT() error {
	jwtSignKeyFile = daemonenv.CAKeyFile()
	jwtVerifyKeyFile = daemonenv.CACertFile()

	if jwtSignKeyFile == "" && jwtVerifyKeyFile == "" {
		return fmt.Errorf("the system/sec/cert-{clustername} listener private_key and certificate must exist.")
	} else if jwtSignKeyFile != "" {
		signBytes, err := ioutil.ReadFile(jwtSignKeyFile)
		if err != nil {
			return err
		}
		if signKey, err = jwt.ParseRSAPrivateKeyFromPEM(signBytes); err != nil {
			return err
		}
		if jwtVerifyKeyFile == "" {
			return errors.Errorf("key file is set to the path of a RSA key. In this case, the certificate file must also be set to the path of the RSA public key.")
		}
		if verifyBytes, err = ioutil.ReadFile(jwtVerifyKeyFile); err != nil {
			return err
		}
		if verifyKey, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes); err != nil {
			return err
		} else {
			if pk, err := ssh.NewPublicKey(verifyKey); err != nil {
				log.Logger.Info().Msgf("  load verify key: %s", err)
			} else {
				finger := ssh.FingerprintLegacyMD5(pk)
				log.Logger.Info().Msgf("  verify key sig: %s", finger)
			}
			TokenAuth = jwtauth.New("RS256", signKey, verifyKey)
		}
	} else {
		return errors.Errorf("the system/sec/cert-{clustername} listener private_key must exist.")
		// If we want to support less secure HMAC token from a static sign key:
		//	TokenAuth = jwtauth.New("HMAC", []byte(jwtSignKey), nil)
	}
	return nil
}

//
// GetToken     godoc
// @Summary      Get a authentication token
// @Description  Get a authentication token from a user's credentials submitted with basic login.
// @Security     BasicAuth
// @Security     BearerAuth
// @Tags         auth
// @Produce      json
// @Success      200  {object}  TokenResponse
// @Failure      403  {string}  string
// @Failure      500  {string}  string  "Internal Server Error"
// @Router       /auth/user/token  [get]
//
func GetToken(w http.ResponseWriter, r *http.Request) {
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
	auth.Append(tokenStrategy, token, user)

	jsonEncode(w, TokenResponse{
		TokenExpireAt: tokenExpireAt,
		Token:         token,
	})
}
