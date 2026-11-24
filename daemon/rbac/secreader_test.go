package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The secreader role is scoped as the guest, operator and admin ones: a
// cluster-wide grant gives it on a namespace, and a token is narrowed to it.
func TestSecReaderIsScoped(t *testing.T) {
	assert.True(t, IsScopedRole(RoleSecReader))

	all := NewGrants("secreader")
	assert.True(t, all.Covers(NewGrant(RoleSecReader, "prod")), "a cluster-wide secreader gives it on a namespace")
	prod := NewGrants("secreader:prod")
	assert.True(t, prod.Covers(NewGrant(RoleSecReader, "prod")))
	assert.False(t, prod.Covers(NewGrant(RoleSecReader, "dev")))
	assert.False(t, prod.Covers(NewGrant(RoleSecReader, "")))

	cases := []struct {
		name    string
		allowed []string
		roles   []Role
		scope   string
		want    Grants
	}{
		{"cluster-wide secreader narrowed to a namespace", []string{"secreader"}, []Role{RoleSecReader}, "prod", NewGrants("secreader:prod")},
		{"secreader of a namespace narrowed to it", []string{"secreader:prod"}, []Role{RoleSecReader}, "prod", NewGrants("secreader:prod")},
		{"secreader of another namespace", []string{"secreader:dev"}, []Role{RoleSecReader}, "prod", nil},
		{"admin of the namespace", []string{"admin:prod"}, []Role{RoleSecReader}, "prod", NewGrants("secreader:prod")},
		{"admin of another namespace", []string{"admin:dev"}, []Role{RoleSecReader}, "prod", nil},
		{"guest is no secreader", []string{"guest:prod"}, []Role{RoleSecReader}, "prod", nil},
		{"secreader with no scope", []string{"secreader:prod", "secreader:dev"}, []Role{RoleSecReader}, "", NewGrants("secreader:prod", "secreader:dev")},
		{"admin with no scope", []string{"admin:prod"}, []Role{RoleSecReader}, "", NewGrants("secreader:prod")},
		{"scope only, guest and secreader", []string{"guest:prod", "secreader:prod"}, nil, "prod", NewGrants("guest:prod", "secreader:prod")},
		{"scope only, secreader alone", []string{"secreader"}, nil, "prod", NewGrants("secreader:prod")},
		{"scope only, admin", []string{"admin:prod", "secreader:prod"}, nil, "prod", NewGrants("admin:prod")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.ElementsMatch(t, c.want, FilterGrantStrings(c.allowed, c.roles, c.scope))
		})
	}
}
