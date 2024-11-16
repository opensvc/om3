package daemonauth

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/token"
	"golang.org/x/crypto/ssh"
)

type (
	// JWTCreator implements CreateUserToken method
	JWTCreator struct{}

	// apiClaims defines api claims
	apiClaims struct {
		Grant []string `json:"grant"`
		*jwt.RegisteredClaims
	}

	// JWTFiler is the interface that groups SignKeyFile and VerifyKeyFile methods
	// for JWT auth.
	JWTFiler interface {
		SignKeyFile() string
		VerifyKeyFile() string
	}
)

var (
	jwtAuth *jwtauth.JWTAuth

	// jwtVerifyKeySign is the jwt verify key signature initialized during initAuthJWT
	jwtVerifyKeySign string
)

func initJWT(i interface{}) (string, auth.Strategy, error) {
	var (
		err       error
		verifyKey *rsa.PublicKey
		name      = "jwt"
	)

	verifyKey, jwtAuth, err = initAuthJWT(i)
	if err != nil {
		return name, nil, err
	}
	validate := func(ctx context.Context, r *http.Request, s string) (info auth.Info, exp time.Time, err error) {
		var tk *jwt.Token

		tk, err = jwt.ParseWithClaims(s, &apiClaims{}, func(token *jwt.Token) (interface{}, error) {
			return verifyKey, nil
		})
		if err != nil {
			return
		}
		claims := tk.Claims.(*apiClaims)
		exp = claims.ExpiresAt.Time

		extensions := authenticatedExtensions("jwt", claims.Grant...)
		info = auth.NewUserInfo(claims.Subject, claims.Subject, nil, *extensions)
		return
	}

	return name, token.New(validate, cache), nil
}

// initAuthJWT initialize auth JWT and returns verify key and *jwtauth.JWTAuth
func initAuthJWT(i interface{}) (*rsa.PublicKey, *jwtauth.JWTAuth, error) {
	var (
		err error

		verifyBytes []byte
		signBytes   []byte

		signKey   *rsa.PrivateKey
		verifyKey *rsa.PublicKey
	)

	f, ok := i.(JWTFiler)
	if !ok {
		return nil, nil, fmt.Errorf("missing sign and verify files")
	}
	var (
		signKeyFile   = f.SignKeyFile()
		verifyKeyFile = f.VerifyKeyFile()
	)
	if signKeyFile == "" && verifyKeyFile == "" {
		return nil, nil, fmt.Errorf("jwt undefined files: sign key and verify key")
	} else if signKeyFile == "" {
		return nil, nil, fmt.Errorf("jwt undefined file: sign key")
		// If we want to support less secure HMAC token from a static sign key:
		//	jwtAuth = jwtauth.New("HMAC", []byte(jwtSignKey), nil)
	} else if verifyKeyFile == "" {
		return nil, nil, fmt.Errorf("jwt undefined file: verify key")
	}

	if signBytes, err = os.ReadFile(signKeyFile); err != nil {
		return nil, nil, fmt.Errorf("%w: jwt sign key file", err)
	}
	if verifyBytes, err = os.ReadFile(verifyKeyFile); err != nil {
		return nil, nil, fmt.Errorf("%w: jwt verify key file", err)
	}
	if signKey, err = jwt.ParseRSAPrivateKeyFromPEM(signBytes); err != nil {
		return nil, nil, fmt.Errorf("%w: parse RSA private key from sign key file content", err)
	}
	if verifyKey, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes); err != nil {
		return nil, nil, fmt.Errorf("%w: parse RSA public key from verify key file content", err)
	}
	if pk, err := ssh.NewPublicKey(verifyKey); err != nil {
		jwtVerifyKeySign = fmt.Sprintf("can't read public key:%s", err)
	} else {
		jwtVerifyKeySign = ssh.FingerprintLegacyMD5(pk)
	}
	return verifyKey, jwtauth.New("RS256", signKey, verifyKey), nil
}

// CreateUserToken implements CreateUserToken interface for JWTCreator.
// empty token is returned if jwtAuth is not initialized
func (*JWTCreator) CreateUserToken(userInfo auth.Info, duration time.Duration, xClaims map[string]interface{}) (tk string, expiredAt time.Time, err error) {
	if jwtAuth == nil {
		return
	}
	expiredAt = time.Now().Add(duration)
	claims := map[string]interface{}{
		"sub":   userInfo.GetUserName(),
		"exp":   expiredAt.Unix(),
		"grant": userInfo.GetExtensions()["grant"],
	}
	for c, v := range xClaims {
		claims[c] = v
	}
	if _, tk, err = jwtAuth.Encode(claims); err != nil {
		return
	}
	return
}
