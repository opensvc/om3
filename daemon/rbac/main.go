package rbac

import (
	"fmt"
	"strings"
)

type (
	Role string

	// Grants is a list of Grant
	Grants []Grant

	// Grant is <role>:<scope>
	Grant string
)

const (
	RoleUndef          Role = ""
	RoleRoot           Role = "root"
	RoleAdmin          Role = "admin"
	RoleGuest          Role = "guest"
	RoleSquatter       Role = "squatter"
	RoleBlacklistAdmin Role = "blacklistadmin"
	RoleHeartbeat      Role = "heartbeat"
	RoleJoin           Role = "join"
	RoleLeave          Role = "leave"
)

var (
	roleMap = map[string]Role{
		"":               RoleUndef,
		"root":           RoleRoot,
		"admin":          RoleAdmin,
		"guest":          RoleGuest,
		"squatter":       RoleSquatter,
		"blacklistadmin": RoleBlacklistAdmin,
		"heartbeat":      RoleHeartbeat,
		"join":           RoleJoin,
		"leave":          RoleJoin,
	}
)

func NewGrants(l ...string) Grants {
	t := make(Grants, len(l))
	for i, s := range l {
		t[i] = Grant(s)
	}
	return t
}

func (t *Role) String() string {
	return string(*t)
}

// HasGrant returns true if any grant of the variadic grants
// is found.
func (t Grants) HasGrant(grants ...Grant) bool {
	return matchGrants(t, grants...)
}

// HasRole returns true if any role of the variadic roles
// is found.
func (t Grants) HasRole(roles ...Role) bool {
	return matchRoles(t, roles...)
}

func (t Grants) Has(role Role, scope string) bool {
	return match(t, role, scope)
}

func formatGrant(role Role, scope string) Grant {
	var s string
	if scope == "" {
		s = fmt.Sprintf("%s", role)
	} else {
		s = fmt.Sprintf("%s:%s", role, scope)
	}
	return Grant(s)
}

func match(userGrants Grants, role Role, scope string) bool {
	grant := formatGrant(role, scope)
	return matchGrants(userGrants, grant)
}

func matchGrants(userGrants Grants, grants ...Grant) bool {
	for _, userGrant := range userGrants {
		for _, grant := range grants {
			if userGrant == grant {
				return true
			}
		}
	}
	return false
}

func matchRoles(userGrants Grants, roles ...Role) bool {
	for _, g := range userGrants {
		userRole, _ := g.Split()
		for _, r := range roles {
			if Role(userRole) == r {
				return true
			}
		}
	}
	return false
}

// SplitGrant extract role and scope from a grant
func SplitGrant(grant Grant) (r Role, ns string) {
	l := strings.SplitN(string(grant), ":", 2)
	r = toRole(l[0])
	if len(l) == 2 {
		ns = l[1]
	}
	return
}

func toRole(s string) Role {
	if v, ok := roleMap[s]; ok {
		return v
	} else {
		return RoleUndef
	}
}

func (t *Grant) Split() (string, string) {
	l := strings.SplitN(string(*t), ":", 2)
	switch len(l) {
	case 2:
		return l[0], l[1]
	default:
		return l[0], ""
	}
}

func (t *Grant) String() string {
	return string(*t)
}

// Roles returns list of defined roles
func Roles() []string {
	l := make([]string, 0)
	for s := range roleMap {
		if s == "" {
			continue
		}
		l = append(l, s)
	}
	return l
}
