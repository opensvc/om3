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
	RoleOperator       Role = "operator"
	RolePrioritizer    Role = "prioritizer"
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
		"operator":       RoleOperator,
		"prioritizer":    RolePrioritizer,
		"guest":          RoleGuest,
		"squatter":       RoleSquatter,
		"blacklistadmin": RoleBlacklistAdmin,
		"heartbeat":      RoleHeartbeat,
		"join":           RoleJoin,
		"leave":          RoleJoin,
	}

	GrantRoot           = NewGrant("root", "")
	GrantSquatter       = NewGrant("squatter", "")
	GrantHeartbeat      = NewGrant("heartbeat", "")
	GrantBlacklistAdmin = NewGrant("blacklistadmin", "")
	GrantJoin           = NewGrant("join", "")
	GrantLeave          = NewGrant("leave", "")
	GrantPrioritizer    = NewGrant("prioritizer", "")
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

func (t Grants) Namespaces(roles ...Role) []string {
	allowAll := false
	allowedRole := make(map[Role]any)
	m := make(map[string]any)
	for _, role := range roles {
		allowedRole[role] = nil
	}
	if len(roles) == 0 {
		allowAll = true
	}
	for _, grant := range t {
		role, namespace := SplitGrant(grant)
		if namespace == "" {
			continue
		}
		if !allowAll {
			if _, ok := allowedRole[role]; !ok {
				continue
			}
		}
		m[namespace] = nil
	}
	l := make([]string, len(m))
	i := 0
	for k := range m {
		l[i] = k
	}
	return l
}

func NewGrant(role Role, scope string) Grant {
	var s string
	if scope == "" {
		s = fmt.Sprintf("%s", role)
	} else {
		s = fmt.Sprintf("%s:%s", role, scope)
	}
	return Grant(s)
}

func match(userGrants Grants, role Role, scope string) bool {
	grant := NewGrant(role, scope)
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

func (t Grants) String() string {
	strList := make([]string, len(t))
	for i := range t {
		strList[i] = t[i].String()
	}
	return strings.Join(strList, " ")
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
