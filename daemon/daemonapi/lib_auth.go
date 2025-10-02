package daemonapi

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonauth"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/util/converters"
)

var (
	errBadRequest = errors.New("bad request")
	errForbidden  = errors.New("forbidden")

	userDB = &object.UsrDB{}

	grantJoin = rbac.GrantJoin.String()
)

// accessTokenDuration parses a duration string, returning a clamped time.Duration or a default duration if input is nil or empty.
func (a *DaemonAPI) accessTokenDuration(s *string) (time.Duration, error) {
	return converters.TDuration{}.TryConvert(s, time.Minute*10, time.Second, time.Hour)
}

// refreshTokenDuration parses a duration string, returning a clamped time.Duration or a default duration if input is nil or empty.
func (a *DaemonAPI) refreshTokenDuration(s *string) (time.Duration, error) {
	return converters.TDuration{}.TryConvert(s, time.Hour*24, time.Second, time.Hour*24*30)
}

func validateRole(r *api.Roles) error {
	if r == nil {
		return nil
	}
	for _, r := range *r {
		role := rbac.Role(r)
		switch role {
		case rbac.RoleJoin:
		case rbac.RoleAdmin:
		case rbac.RoleBlacklistAdmin:
		case rbac.RoleGuest:
		case rbac.RoleHeartbeat:
		case rbac.RoleLeave:
		case rbac.RoleOperator:
		case rbac.RoleRoot:
		case rbac.RoleSquatter:
		case rbac.RoleUndef:
		default:
			return fmt.Errorf("unexpected role %s: %w", role, errBadRequest)
		}
	}
	return nil
}

// filterGrant filters the requested roles and scopes based on the allowed grants.
func filterGrant(allowed []string, rolesP *api.Roles, scopeP *string) (grants []string, err error) {
	if err := validateRole(rolesP); err != nil {
		return nil, err
	}
	var roles []rbac.Role
	if rolesP == nil {
		added := make(map[string]struct{})
		for _, v := range allowed {
			if _, ok := added[v]; ok {
				continue
			}
			g := rbac.Grant(v)
			r, _ := g.Split()
			roles = append(roles, rbac.Role(r))
		}
	} else {
		for _, r := range *rolesP {
			roles = append(roles, rbac.Role(r))
		}
	}
	allowedGrants := rbac.NewGrants(allowed...)
	var scope string
	if scopeP != nil {
		scope = *scopeP
	}
	roleDone := make(map[rbac.Role]bool)
	for _, role := range roles {
		if _, ok := roleDone[role]; ok {
			continue
		}
		if role == rbac.RoleUndef {
			continue
		}
		grant := rbac.NewGrant(role, scope)
		if allowedGrants.HasGrant(grant) {
			grants = append(grants, grant.String())
		} else if allowedGrants.Has(rbac.RoleRoot, "") {
			// TODO: clarify this rule
			grants = append(grants, grant.String())
		} else {
			err = fmt.Errorf("refused grant %s: %w", grant, errForbidden)
			return
		}
		roleDone[role] = true
	}
	return
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

func (a *DaemonAPI) createToken(username, tokenUse string, duration time.Duration, claims map[string]any) (string, time.Time, error) {
	if username == "" {
		return "", time.Time{}, fmt.Errorf("username is empty")
	}
	if tokenUse == "" {
		return "", time.Time{}, fmt.Errorf("token use is empty")
	}
	xc := make(map[string]any)
	xc["sub"] = username
	xc["token_use"] = tokenUse
	xc["iss"] = a.localhost
	for c, v := range claims {
		xc[c] = v
	}

	return a.JWTcreator.CreateToken(duration, xc)
}

func (a *DaemonAPI) createAccessToken(ctx echo.Context, username string, duration time.Duration, pRole *api.Roles, pScope *string) (d api.AuthAccessToken, err error) {
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

	if grantL, err := filterGrant(grantL, pRole, pScope); err != nil {
		return d, fmt.Errorf("filter grant: %w", err)
	} else if claims, err := a.xClaimForGrants(grantL); err != nil {
		return d, fmt.Errorf("create claims: %w", err)
	} else if tk, exp, err := a.createToken(username, "access", duration, claims); err != nil {
		return d, fmt.Errorf("create token: %w", err)
	} else {
		d.AccessToken = tk
		d.AccessExpiredAt = exp
		return d, nil
	}
}
