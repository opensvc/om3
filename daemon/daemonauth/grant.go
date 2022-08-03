package daemonauth

import (
	"net/http"
	"strings"

	"github.com/shaj13/go-guardian/v2/auth"
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
)

func roleHasNamespace(role Role) bool {
	switch role {
	case RoleRoot, RoleBlacklistAdmin, RoleHeartbeat:
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

func (t Grants) List() []string {
	l := make([]string, len(t))
	for i, g := range t {
		l[i] = string(g)
	}
	return l
}

func (t Grants) Extensions() auth.Extensions {
	ext := make(auth.Extensions)
	ext["grant"] = t.List()
	return ext
}

func (t Grants) HasRoot() bool {
	for _, g := range t {
		if g.Role() == RoleRoot {
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
	default:
		return RoleUndef
	}
}

func (t Grant) NamespaceSelector() string {
	_, selector := t.split()
	return selector
}
