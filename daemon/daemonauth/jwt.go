package daemonauth

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	// JWTCreator implements CreateUserToken method
	JWTCreator struct{}

	// apiClaims defines api claims.
	//
	// RegisteredClaims is embedded by value: as a pointer it stays nil
	// for a token that carries none of the registered claims, and the
	// validator dereferences it to read the expiration.
	apiClaims struct {
		Grant    []string `json:"grant"`
		TokenUse string   `json:"token_use"`
		jwt.RegisteredClaims
	}

	// JWTFiler is the interface that groups SignKeyFile and VerifyKeyFile methods
	// for JWT auth.
	JWTFiler interface {
		SignKeyFile() string
		VerifyKeyFile() string
	}
)

var (
	jwtSignKey atomic.Pointer[rsa.PrivateKey] // Stores the RSA private key for signing

	// jwtVerifyKeySign is the jwt verify key signature initialized during initAuthJWT
	jwtVerifyKeySign string
)

const (
	// TkUseClaim is a constant used as the key to identify the token usage type in claims or authentication context.
	TkUseClaim = "token_use"

	// TkUseAccess represents the token usage type for access tokens.
	TkUseAccess = "access"

	// TkUseRefresh represents the token usage type for refresh tokens.
	TkUseRefresh = "refresh"

	// TkUseProxy represents the token usage type for proxy tokens.
	TkUseProxy = "proxy"
)

// bearerToken returns the token of an Authorization: Bearer header.
//
// The scheme is matched case sensitively, as the previous implementation
// did, and as every client of this api sends it.
func bearerToken(r *http.Request) (string, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	scheme, tk, found := strings.Cut(header, " ")
	if !found || scheme != "Bearer" || tk == "" {
		return "", fmt.Errorf("request has no bearer token")
	}
	return tk, nil
}

// jwtStrategy authenticates the tokens this cluster issues itself, the
// ones signed with the cluster CA private key.
type jwtStrategy struct {
	verifyKey *rsa.PublicKey
}

func (t *jwtStrategy) Authenticate(_ context.Context, r *http.Request) (*Info, error) {
	tkString, err := bearerToken(r)
	if err != nil {
		return nil, fmt.Errorf("strategies/jwt: %w", err)
	}
	key := cacheKey(StrategyJWT, tkString)
	if info, ok := authCache.get(key); ok {
		return info, nil
	}
	claims := &apiClaims{}
	// Without WithValidMethods a token naming a signing method we never
	// issue is handed to that method's verifier with our rsa key, and
	// what happens next is that package's business rather than ours.
	// WithExpirationRequired refuses the token that has no exp claim,
	// which is the one that never stops being valid.
	if _, err := jwt.ParseWithClaims(tkString, claims,
		func(*jwt.Token) (interface{}, error) { return t.verifyKey, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithExpirationRequired(),
	); err != nil {
		return nil, fmt.Errorf("strategies/jwt: %w", err)
	}
	info := &Info{
		Username: claims.Subject,
		Strategy: StrategyJWT,
		Issuer:   claims.Issuer,
		TokenUse: claims.TokenUse,
		Grants:   claims.Grant,
	}
	authCache.set(key, info, time.Until(claims.ExpiresAt.Time))
	return info, nil
}

// initJWT initializes the JWT authentication strategy using provided configuration and context.
// It returns the strategy name ("jwt"), an instance of the Strategy, and any error encountered.
func initJWT(_ context.Context, i interface{}) (string, Strategy, error) {
	name := "jwt"
	verifyKey, signKey, err := initAuthJWT(i)
	if err != nil {
		return name, nil, err
	}
	jwtSignKey.Store(signKey)
	return name, &jwtStrategy{verifyKey: verifyKey}, nil
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

// CreateToken implements CreateToken interface for `daemonapi.JWTCreater`.
// It generates a JWT with the specified duration and custom claims,
// returning the token, expiration time, and error if any.
func (*JWTCreator) CreateToken(duration time.Duration, xClaims map[string]interface{}) (tk string, expiredAt time.Time, err error) {
	signKey := jwtSignKey.Load()
	if signKey == nil {
		// This returned an empty token and no error, which the caller
		// then sent as its bearer and read the 401 back as a credentials
		// problem, at the other end of the cluster from the node whose
		// jwt strategy failed to initialize.
		err = fmt.Errorf("jwt sign key is not loaded")
		return
	}
	expiredAt = time.Now().Add(duration)
	allClaims := make(jwt.MapClaims)
	allClaims["exp"] = expiredAt.Unix()

	for c, v := range xClaims {
		allClaims[c] = v
	}

	// Create a new token with RS256 signing method and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, allClaims)

	// Sign the token using the RSA private key
	if tk, err = token.SignedString(signKey); err != nil {
		return
	}

	if tk == "" {
		err = fmt.Errorf("empty token")
	}
	return
}

// nodeTokenDuration is how long a node to node token is valid. It covers
// one request, and the clock difference between the node that signs it
// and the peer that verifies it.
const nodeTokenDuration = 5 * time.Second

// CreateNodeToken returns a bearer token authenticating the local node to
// its peers, with the root grant.
//
// This is what node to node api calls use. They used to send the cluster
// secret as a basic auth password, which the peer compared to its own
// copy: the secret is also the key every sec object is encrypted with, so
// a call that reached the wrong listener handed over more than access to
// one config file. The token is signed with the cluster CA private key,
// which every node of the cluster has, so any peer verifies it, and it is
// worthless five seconds later.
func CreateNodeToken() (string, error) {
	nodename := hostname.Hostname()
	tk, _, err := (&JWTCreator{}).CreateToken(nodeTokenDuration, map[string]any{
		"sub":      nodename,
		"iss":      nodename,
		"grant":    []string{rbac.GrantRoot.String()},
		TkUseClaim: TkUseAccess,
	})
	if err != nil {
		return "", fmt.Errorf("create node token: %w", err)
	}
	return tk, nil
}
