package daemonapi

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	errBadRequest = errors.New("bad request")
	errForbidden  = errors.New("forbidden")

	userDB = &object.UsrDB{}

	grantJoin = rbac.GrantJoin.String()
)

// canCreateAccessToken determines whether an access token can be created based
// on the token type (refresh token) or authentication strategy (UX or User).
func (a *DaemonAPI) canCreateAccessToken(ctx echo.Context) bool {
	if s, ok := ctx.Get(daemonauth.TkUseClaim).(string); ok && s == daemonauth.TkUseRefresh {
		return true
	}
	strategy := strategyFromContext(ctx)
	switch strategy {
	case daemonauth.StrategyUX:
	case daemonauth.StrategyUser:
	default:
		return false
	}
	return true
}

// canCreateRefreshToken determines if a refresh token can be created based
// on the authentication strategy from the context: It needs StrategyUX or
// StrategyUser.
func (a *DaemonAPI) canCreateRefreshToken(ctx echo.Context) bool {
	strategy := strategyFromContext(ctx)
	switch strategy {
	case daemonauth.StrategyUX:
	case daemonauth.StrategyUser:
	default:
		return false
	}
	return true
}

// accessTokenDuration parses a duration string, returning a clamped time.Duration or a default duration if input is nil or empty.
func (a *DaemonAPI) accessTokenDuration(s *string) (time.Duration, error) {
	return converters.DurationWithDefaultMinMax(s, time.Minute*10, time.Second, 24*time.Hour)
}

// refreshTokenDuration parses a duration string, returning a clamped time.Duration or a default duration if input is nil or empty.
func (a *DaemonAPI) refreshTokenDuration(s *string) (time.Duration, error) {
	return converters.DurationWithDefaultMinMax(s, time.Hour*24, time.Second, time.Hour*24*30)
}

func validateRole(r *api.Roles) error {
	if r == nil {
		return nil
	}
	for _, r := range *r {
		role := rbac.ParseRole(string(r))
		if role == rbac.RoleUndef {
			return fmt.Errorf("unexpected role %s: %w", role, errBadRequest)
		}
	}
	return nil
}

// filterGrant filters the requested roles and scopes based on the allowed grants.
func filterGrant(allowed []string, rolePtr *api.Roles, scopePtr *string) ([]string, error) {
	var (
		scope string
		roles []rbac.Role
	)

	if err := validateRole(rolePtr); err != nil {
		return nil, err
	}

	if rolePtr != nil {
		for _, e := range *rolePtr {
			if role := rbac.ParseRole(string(e)); role != rbac.RoleUndef {
				roles = append(roles, role)
			}
		}
	}
	if scopePtr != nil {
		scope = *scopePtr
	}

	grants := rbac.FilterGrantStrings(allowed, roles, scope)
	return grants.AsStringList(), nil
}

// xClaims returns new user and Claims from p and current user
func (a *DaemonAPI) xClaimForGrants(grants []string) (map[string]interface{}, error) {
	xc := map[string]interface{}{
		"iss": a.localhost,
	}
	for _, g := range grants {
		if g == grantJoin {
			var b []byte
			filename := daemonenv.CertChainFile()
			b, err := os.ReadFile(filename)
			if err != nil {
				return xc, err
			}
			xc["ca"] = string(b)
		}
	}
	if len(grants) > 0 {
		xc["grant"] = append([]string{}, grants...)
	}
	return xc, nil
}

func (a *DaemonAPI) createToken(username, tkUseValue string, duration time.Duration, claims map[string]any) (string, time.Time, error) {
	if username == "" {
		return "", time.Time{}, fmt.Errorf("username is empty")
	}
	if tkUseValue == "" {
		return "", time.Time{}, fmt.Errorf("token use is empty")
	}
	xc := make(map[string]any)
	xc["sub"] = username
	xc[daemonauth.TkUseClaim] = tkUseValue
	xc["iss"] = a.localhost
	for c, v := range claims {
		xc[c] = v
	}

	return a.JWTcreator.CreateToken(duration, xc)
}

func (a *DaemonAPI) createAccessToken(ctx echo.Context, username string, duration time.Duration, rolePtr *api.Roles, scopePtr *string) (d api.AuthAccessToken, err error) {
	var grantL []string
	if username == "root" && strategyFromContext(ctx) == daemonauth.StrategyUX {
		grants := grantsFromContext(ctx)
		for _, g := range grants {
			grantL = append(grantL, g.String())
		}
	} else if grantL, err = userDB.GrantsFromUsername(username); err != nil {
		err := errors.Join(errForbidden, fmt.Errorf("user grants for username '%s': %w", username, err))
		return d, err
	}

	if grantL, err := filterGrant(grantL, rolePtr, scopePtr); err != nil {
		return d, fmt.Errorf("filter grant: %w", err)
	} else if len(grantL) == 0 {
		return d, errors.Join(errForbidden, fmt.Errorf("no grant matching role and scope for username '%s'", username))
	} else if claims, err := a.xClaimForGrants(grantL); err != nil {
		return d, fmt.Errorf("create claims: %w", err)
	} else if tk, exp, err := a.createToken(username, daemonauth.TkUseAccess, duration, claims); err != nil {
		return d, fmt.Errorf("create token: %w", err)
	} else {
		d.AccessToken = tk
		d.AccessExpiredAt = exp
		return d, nil
	}
}

func (a *DaemonAPI) createAccessTokenWithGrants(username string, duration time.Duration, tkUse string, grantL []string) (d api.AuthAccessToken, err error) {
	if username == "" {
		return d, fmt.Errorf("username is empty")
	}
	if claims, err := a.xClaimForGrants(grantL); err != nil {
		return d, fmt.Errorf("create claims: %w", err)
	} else if tk, exp, err := a.createToken(username, tkUse, duration, claims); err != nil {
		return d, fmt.Errorf("create token: %w", err)
	} else {
		d.AccessToken = tk
		d.AccessExpiredAt = exp
		return d, nil
	}
}
