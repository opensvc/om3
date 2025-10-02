package daemonapi

import (
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/util/converters"
)

var (
	errBadRequest = errors.New("bad request")
)

// accessTokenDuration parses a duration string, returning a clamped time.Duration or a default duration if input is nil or empty.
func (a *DaemonAPI) accessTokenDuration(s *string) (time.Duration, error) {
	return converters.TDuration{}.TryConvert(s, time.Minute*10, time.Second, time.Hour)
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
