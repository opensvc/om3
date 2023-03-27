package daemonauth

import (
	"net/http"
	"strings"

	"github.com/shaj13/go-guardian/v2/auth"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
)

type (
	// Grants is a list of <role>:<namespace pattern>
	Grants []Grant
	Grant  string
	Role   string
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

func roleHasNamespace(role Role) bool {
	switch role {
	case RoleRoot, RoleBlacklistAdmin, RoleHeartbeat, RoleJoin, RoleLeave:
		return false
	default:
		return true
	}
}

// UserGrants return the grants stored in the request context. This is a helper for
// handlers.
func UserGrants(r *http.Request) Grants {
	u := User(r)
	exts := u.GetExtensions()
	return NewGrants(exts["grant"]...)
}

func NewGrants(l ...string) Grants {
	t := make(Grants, len(l))
	for i, s := range l {
		t[i] = Grant(s)
	}
	return t
}

// List return the grants in the string slice format
func (t Grants) List() []string {
	l := make([]string, len(t))
	for i, g := range t {
		l[i] = string(g)
	}
	return l
}

// Extensions return the grants in go-guardian Extensions format.
func (t Grants) Extensions() auth.Extensions {
	ext := make(auth.Extensions)
	ext["grant"] = t.List()
	return ext
}

// HasRoot returns true if any of the grants is "root"
func (t Grants) HasRoot() bool {
	for _, g := range t {
		if g.Role() == RoleRoot {
			return true
		}
	}
	return false
}

// HasAnyRole returns true if t has any role
func (t Grants) HasAnyRole(roles ...Role) bool {
	for _, g := range t {
		for _, r := range roles {
			if g.Role() == r {
				return true
			}
		}
	}
	return false
}

// FilterPaths return the list of path.T allowed by grants of <role>
func (t Grants) FilterPaths(r *http.Request, role Role, l path.L) path.L {
	fl := make(path.L, 0)
	for _, p := range l {
		if t.MatchPath(r, role, p) {
			fl = append(fl, p)
		}
	}
	return fl
}

// MatchPath returns true if path <p> is allowed by grants of <role>
func (t Grants) MatchPath(r *http.Request, role Role, p path.T) bool {
	for _, grant := range t {
		if grant.Match(r, role, p.Namespace) {
			return true
		}
	}
	return false
}

// Match returns true if the path <p> is allowed by this grant
func (t Grant) Match(r *http.Request, role Role, namespace string) bool {
	if t.Role() != role {
		return false
	}
	if namespace == "" {
		return true
	}
	if t.NamespaceSelector() == "" {
		return true
	}
	for _, ns := range t.Namespaces(r) {
		if ns == namespace {
			return true
		}
	}
	return false
}

func (t Grant) String() string {
	return string(t)
}

func (t Grant) split() (string, string) {
	l := strings.SplitN(string(t), ":", 2)
	switch len(l) {
	case 2:
		return l[0], l[1]
	default:
		return l[0], ""
	}
}

func (t Grant) Role() Role {
	s, _ := t.split()
	switch s {
	case "root":
		return RoleRoot
	case "admin":
		return RoleAdmin
	case "guest":
		return RoleGuest
	case "squatter":
		return RoleSquatter
	case "blacklistadmin":
		return RoleBlacklistAdmin
	case "heartbeat":
		return RoleHeartbeat
	case "join":
		return RoleJoin
	case "leave":
		return RoleLeave
	default:
		return RoleUndef
	}
}

func (t Grant) NamespaceSelector() string {
	_, selector := t.split()
	return selector
}

// Namespaces returns the list of unique namespace names found in the
// daemon data.
func (t Grant) Namespaces(r *http.Request) []string {
	return object.StatusData.GetPaths().Namespaces()
}
