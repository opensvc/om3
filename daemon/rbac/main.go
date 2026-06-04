package rbac

import (
	"fmt"
	"slices"
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
	// roleMap maps string identifiers to their corresponding Role constants,
	// defining the available roles and their associations.
	// public roles must be added to the api.yaml Role definition.
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

func (t Grants) WithRole(roles ...Role) (grants Grants) {
	for _, grant := range t {
		if r := grant.Role(); slices.Contains(roles, r) {
			grants = append(grants, grant)
		}
	}
	return
}

// HasRoleOn checks if any of the specified roles with the given scope exist in the Grants.
func (t Grants) HasRoleOn(scope string, roles ...Role) bool {
	grants := make(Grants, len(roles))
	for i, role := range roles {
		grants[i] = NewGrant(role, scope)
	}
	return matchGrants(t, grants...)
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
		i++
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
	r = ParseRole(l[0])
	if len(l) == 2 {
		ns = l[1]
	}
	return
}

func ParseRole(s string) Role {
	if v, ok := roleMap[s]; ok {
		return v
	} else {
		return RoleUndef
	}
}

func (t *Grant) Role() Role {
	role, _, _ := strings.Cut(string(*t), ":")
	return Role(role)
}

func (t *Grant) Scope() string {
	_, scope, _ := strings.Cut(string(*t), ":")
	return scope
}

func (t *Grant) Split() (string, string) {
	role, scope, _ := strings.Cut(string(*t), ":")
	return role, scope
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

func (t Grants) AsStringList() []string {
	l := make([]string, 0, len(t))
	for _, g := range t {
		l = append(l, string(g))
	}
	return l
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

func IsScopedRole(role Role) bool {
	switch role {
	case RoleAdmin, RoleOperator, RoleGuest:
		return true
	default:
		return false
	}
}

func (t Grants) FilterRole(role Role) (grants Grants) {
	grant := NewGrant(role, "")
	if t.HasRole(RoleRoot) {
		return append(grants, grant)
	}
	if role == RoleGuest {
		if t.HasRoleOn("", RoleOperator, RoleAdmin, RoleGuest) {
			return append(grants, grant)
		}
		for _, namespace := range t.Namespaces(RoleAdmin, RoleOperator, RoleGuest) {
			grants = append(grants, NewGrant(role, namespace))
		}
		return
	}
	if role == RoleOperator {
		if t.HasRoleOn("", RoleAdmin, RoleOperator) {
			return append(grants, grant)
		}
		for _, namespace := range t.Namespaces(RoleAdmin, RoleOperator) {
			grants = append(grants, NewGrant(role, namespace))
		}
		return
	}
	if role == RoleAdmin {
		if t.HasRoleOn("", RoleAdmin) {
			return append(grants, grant)
		}
		for _, namespace := range t.Namespaces(RoleAdmin) {
			grants = append(grants, NewGrant(role, namespace))
		}
		return
	}
	if t.HasGrant(grant) {
		return append(grants, grant)
	}

	return
}

func (t Grants) FilterGrant(grant Grant) (grants Grants) {
	role := grant.Role()
	scope := grant.Scope()
	if t.HasRole(RoleRoot) {
		return append(grants, grant)
	}
	if t.HasGrant(grant) {
		return append(grants, grant)
	}
	if role == RoleGuest {
		if t.HasRoleOn(scope, RoleOperator, RoleAdmin) {
			return append(grants, grant)
		}
		if t.HasRoleOn("", RoleOperator, RoleAdmin) {
			return append(grants, grant)
		}
	}
	if role == RoleOperator {
		if t.HasRoleOn(scope, RoleAdmin) {
			return append(grants, grant)
		}
		if t.HasRoleOn("", RoleAdmin) {
			return append(grants, grant)
		}
	}
	return
}

func (t Grants) FilterScope(scope string) (grants Grants) {
	if t.HasRole(RoleRoot) {
		// give admin role to all given scope
		grant := NewGrant(RoleAdmin, scope)
		grants = append(grants, grant)
		return
	}
	if t.Has(RoleAdmin, scope) || t.Has(RoleAdmin, "") {
		grant := NewGrant(RoleAdmin, scope)
		grants = append(grants, grant)
		return
	}
	if t.Has(RoleOperator, scope) || t.Has(RoleOperator, "") {
		grant := NewGrant(RoleOperator, scope)
		grants = append(grants, grant)
		return
	}
	if t.Has(RoleGuest, scope) || t.Has(RoleGuest, "") {
		grant := NewGrant(RoleGuest, scope)
		grants = append(grants, grant)
		return
	}
	return
}

func FilterGrantStrings(allowed []string, roles []Role, scope string) Grants {
	allowedGrants := NewGrants(allowed...)
	var grants Grants

	// Role       Scope     Grants                           Filtered Grants
	// ---        ---       ---                              ---
	// nil        nil       root                             root
	// nil        nil       admin                            admin
	// nil        nil       admin:ns1,admin:ns2,guest:ns3    admin:ns1,admin:ns2,guest:ns3
	if len(roles) == 0 && scope == "" {
		return allowedGrants
	}

	// Role       Scope     Grants                           Filtered Grants
	// ---        ---       ---                              ---
	// admin      ns2       root                             admin:ns2
	// admin      ns2       admin                            admin:ns2
	// guest      ns2       admin                            guest:ns2
	// admin      ns2       admin:ns1,admin:ns2,guest:ns3    admin:ns2
	// admin      ns3       admin:ns1,admin:ns2,guest:ns3
	// guest      ns2       admin:ns1,admin:ns2,guest:ns3    guest:ns2
	if len(roles) > 0 && scope != "" {
		for _, role := range roles {
			grant := NewGrant(role, scope)
			grants = append(grants, allowedGrants.FilterGrant(grant)...)
		}
		return grants
	}

	// Role       Scope     Grants                           Filtered Grants
	// ---        ---       ---                              ---
	// root       nil       root                             root
	// admin      nil       root                             admin
	// root       nil       admin
	// admin      nil       admin                            admin
	// guest      nil       admin                            guest
	// root       nil       admin:ns1,admin:ns2,guest:ns3
	// admin      nil       admin:ns1,admin:ns2,guest:ns3    admin:ns1,admin:ns2
	// guest      nil       admin:ns1,admin:ns2,guest:ns3    guest:ns3
	if len(roles) > 0 && scope == "" {
		for _, role := range roles {
			grants = append(grants, allowedGrants.FilterRole(role)...)
		}
		return grants
	}

	// Role       Scope     Grants                           Filtered Grants
	// ---        ---       ---                              ---
	// nil        ns1       root                             admin:ns1
	// nil        ns1       guest                            guest:ns1
	// nil        ns1       admin:ns1,admin:ns2,guest:ns3    admin:ns1
	// nil        ns3       admin:ns1,admin:ns2,guest:ns3    guest:ns3
	if len(roles) == 0 && scope != "" {
		grants = append(grants, allowedGrants.FilterScope(scope)...)
		return grants
	}

	return grants
}
