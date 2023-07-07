package rbac

import (
	"strings"
)

type (
	Role string

	// Grants is a list of Grant
	Grants []Grant

	// Grant is <role>:<namespace pattern>
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

func MatchRoles(userGrants Grants, roles ...Role) bool {
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

// SplitGrant extract role and namespace pattern from a grant
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
