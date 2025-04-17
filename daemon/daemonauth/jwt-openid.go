package daemonauth

import (
	"context"
	"net/http"
	"time"

	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/claims"
	"github.com/shaj13/go-guardian/v2/auth/strategies/oauth2"
	"github.com/shaj13/go-guardian/v2/auth/strategies/oauth2/jwt"
	"github.com/shaj13/go-guardian/v2/auth/strategies/token"
	"github.com/shaj13/libcache"
)

type (
	k struct {
		baseStrategy auth.Strategy
	}

	IDTokenGrant struct {
		*jwt.IDToken
		Grant []string `json:"grant"`
	}
)

func (i IDTokenGrant) New() oauth2.ClaimsResolver {
	return &IDTokenGrant{
		IDToken: &jwt.IDToken{
			Info:     auth.NewUserInfo("", "", []string{}, make(auth.Extensions)),
			Standard: new(claims.Standard),
		},
		Grant: make([]string, 0),
	}
}

func (i IDTokenGrant) Verify(options claims.VerifyOptions) error {
	return i.IDToken.Verify(options)
}

func (i IDTokenGrant) Resolve() auth.Info {
	return i
}

// Authenticate verifies user credentials using the base strategy and maps
// the ID token to user information with extensions with grant.
func (s *k) Authenticate(ctx context.Context, r *http.Request) (auth.Info, error) {
	info, err := s.baseStrategy.Authenticate(ctx, r)
	if err != nil {
		return nil, err
	}
	tk := info.(IDTokenGrant)
	extensions := authenticatedExtensions(StrategyJWTOpenID, tk.Issuer, tk.Grant...)
	return auth.NewUserInfo(info.GetUserName(), info.GetUserName(), nil, *extensions), nil
}

// initJWTOpenID initializes the JWT OpenID authentication strategy from the
// provided interface implementation.
// It checks for the JwksUri method in the input, creates a JWT strategy when valid,
// and utilizes a FIFO cache.
// Returns the strategy name, constructed strategy, or an error if initialization fails.
func initJWTOpenID(i interface{}) (string, auth.Strategy, error) {
	type jwksUrier interface {
		JwksUri() (string, error)
	}

	o, ok := i.(jwksUrier)
	if !ok {
		return StrategyJWTOpenID, nil, nil
	}
	jwksUri, err := o.JwksUri()
	if err != nil {
		return StrategyJWTOpenID, nil, err
	}
	if jwksUri == "" {
		return StrategyJWTOpenID, nil, nil
	}
	cache := libcache.FIFO.New(100)
	cache.SetTTL(time.Second)
	verifyOptionTime := func() time.Time {
		return time.Now().Add(time.Second * 5)
	}
	opt := []auth.Option{
		token.SetParser(token.AuthorizationParser("Bearer")),
		jwt.SetVerifyOptions(claims.VerifyOptions{Time: verifyOptionTime}),
		jwt.SetClaimResolver(&IDTokenGrant{}),
	}
	strategy := jwt.New(jwksUri, cache, opt...)
	return StrategyJWTOpenID, &k{baseStrategy: strategy}, nil
}
