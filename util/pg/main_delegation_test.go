package pg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A group and the same group delegated to a user are two groups of the
// hierarchy, each made and capped on its own, so the manager keeps both.
func TestTheManagerKeepsAGroupAndItsDelegatedCopy(t *testing.T) {
	m := &Mgr{configs: make(map[string]*Config)}
	d := Delegation{Root: "/user.slice/user-1001.slice/user@1001.service", UID: 1001, GID: 1001}
	obj := &Config{ID: "/opensvc.slice/opensvc-svc.pod10.slice", CPUShares: "200"}
	res := &Config{ID: "/opensvc.slice/opensvc-svc.pod10.slice/opensvc-svc.pod10-container.0.slice"}
	m.Register(obj)
	m.Register(res.Delegated(d))
	m.Register(obj.Delegated(d))
	m.Register(obj.Delegated(d))
	assert.Len(t, m.configs, 3, "the delegated copy of the object group is kept once, beside the object group")

	ancestors := m.Ancestors(res.ID)
	if assert.Len(t, ancestors, 1, "the delegated copies are not ancestors of anything: they are placed, not configured") {
		assert.Equal(t, obj.ID, ancestors[0].ID)
		assert.Nil(t, ancestors[0].Delegation)
		assert.Equal(t, "200", ancestors[0].CPUShares, "the ancestor carries its cappings to its delegated copy")
	}
}

// Ancestors are the groups a group is nested in, not the ones sharing a
// prefix of its name.
func TestAncestorsAreNestingGroupsOnly(t *testing.T) {
	m := &Mgr{configs: make(map[string]*Config)}
	for _, id := range []string{
		"/opensvc.slice/opensvc-svc.pod1.slice",
		"/opensvc.slice/opensvc-svc.pod10.slice",
		"/opensvc.slice/opensvc-svc.pod10.slice/opensvc-svc.pod10-container.0.slice",
	} {
		m.Register(&Config{ID: id})
	}
	l := m.Ancestors("/opensvc.slice/opensvc-svc.pod10.slice/opensvc-svc.pod10-container.1.slice")
	if assert.Len(t, l, 1) {
		assert.Equal(t, "/opensvc.slice/opensvc-svc.pod10.slice", l[0].ID, "pod1 prefixes pod10 by name, not by nesting")
	}
}
