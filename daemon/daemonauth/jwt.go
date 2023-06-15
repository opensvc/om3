package daemonauth

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/jwtauth/v5"
	"github.com/rs/zerolog/log"
	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/token"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/daemon/daemonenv"
)

type (
	Claims map[string]interface{}

	// ApiClaims defines api claims
	ApiClaims struct {
		Grant Grants `json:"grant"`
		*jwt.StandardClaims
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

	NotImplementedError = errors.New("token based authentication is not configured")
)

func initToken() auth.Strategy {
	log.Logger.Info().Msg("init token auth strategy")
	if err := initJWT(); err != nil {
		log.Logger.Error().Err(err).Msg("init token auth strategy")
		return nil
	}
	tokenStrategy = token.New(validateToken, cache)
	return tokenStrategy
}

func validateToken(ctx context.Context, r *http.Request, s string) (info auth.Info, exp time.Time, err error) {
	var (
		tk *jwt.Token
	)

	tk, err = jwt.ParseWithClaims(s, &ApiClaims{}, func(token *jwt.Token) (interface{}, error) {
		return verifyKey, nil
	})
	if err != nil {
		return
	}
	claims := tk.Claims.(*ApiClaims)
	exp = time.Unix(claims.ExpiresAt, 0)

	extensions := claims.Grant.Extensions()
	extensions.Add("strategy", "jwt")
	info = auth.NewUserInfo(claims.Subject, claims.Subject, nil, extensions)
	return
}

func initJWT() error {
	jwtSignKeyFile = daemonenv.CAKeyFile()
	jwtVerifyKeyFile = daemonenv.CACertChainFile()

	if jwtSignKeyFile == "" && jwtVerifyKeyFile == "" {
		return fmt.Errorf("the system/sec/cert listener private_key and certificate must exist")
	} else if jwtSignKeyFile != "" {
		signBytes, err := os.ReadFile(jwtSignKeyFile)
		if err != nil {
			return err
		}
		if signKey, err = jwt.ParseRSAPrivateKeyFromPEM(signBytes); err != nil {
			return err
		}
		if jwtVerifyKeyFile == "" {
			return fmt.Errorf("key file is set to the path of a RSA key. In this case, the certificate file must also be set to the path of the RSA public key")
		}
		if verifyBytes, err = os.ReadFile(jwtVerifyKeyFile); err != nil {
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
		return fmt.Errorf("the system/sec/cert listener private_key must exist")
		// If we want to support less secure HMAC token from a static sign key:
		//	TokenAuth = jwtauth.New("HMAC", []byte(jwtSignKey), nil)
	}
	return nil
}

func CreateUserToken(userInfo auth.Info, duration time.Duration, xClaims Claims) (tk string, expiredAt time.Time, err error) {
	if TokenAuth == nil {
		err = NotImplementedError
		return
	}
	expiredAt = time.Now().Add(duration)
	claims := Claims{
		"sub":   userInfo.GetUserName(),
		"exp":   expiredAt.Unix(),
		"grant": userInfo.GetExtensions()["grant"],
	}
	for c, v := range xClaims {
		claims[c] = v
	}
	if _, tk, err = TokenAuth.Encode(claims); err != nil {
		return
	}
	return
}
