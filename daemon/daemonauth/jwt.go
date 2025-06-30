package daemonauth

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"time"

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
	jwtSignKey *rsa.PrivateKey // Stores the RSA private key for signing
	// jwtVerifyKeySign is the jwt verify key signature initialized during initAuthJWT
	jwtVerifyKeySign string
)

func initJWT(i interface{}) (string, auth.Strategy, error) {
	var (
		err       error
		verifyKey *rsa.PublicKey
		name      = "jwt"
	)

	var signKey *rsa.PrivateKey // Temporary variable to capture the signKey
	verifyKey, signKey, err = initAuthJWT(i)
	if err != nil {
		return name, nil, err
	}
	jwtSignKey = signKey // Assign to the global variable
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
		iss := claims.Issuer

		extensions := authenticatedExtensions(StrategyJWT, iss, claims.Grant...)
		info = auth.NewUserInfo(claims.Subject, claims.Subject, nil, *extensions)
		return
	}

	return name, token.New(validate, cache), nil
}

// initAuthJWT initialize auth JWT and returns verify key and sign key
func initAuthJWT(i interface{}) (*rsa.PublicKey, *rsa.PrivateKey, error) {
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
	return verifyKey, signKey, nil
}

// CreateUserToken implements CreateUserToken interface for JWTCreator.
// empty token is returned if jwtSignKey is not initialized
func (*JWTCreator) CreateUserToken(userInfo auth.Info, duration time.Duration, xClaims map[string]interface{}) (tk string, expiredAt time.Time, err error) {
	if jwtSignKey == nil {
		return
	}
	expiredAt = time.Now().Add(duration)
	allClaims := make(jwt.MapClaims)
	allClaims["sub"] = userInfo.GetUserName()
	allClaims["exp"] = expiredAt.Unix()
	allClaims["grant"] = userInfo.GetExtensions()["grant"]

	for c, v := range xClaims {
		allClaims[c] = v
	}

	// Create a new token with RS256 signing method and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, allClaims)

	// Sign the token using the RSA private key
	if tk, err = token.SignedString(jwtSignKey); err != nil {
		return
	}
	return
}
