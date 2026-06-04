package rbac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterGrantStrings(t *testing.T) {
	// Test case 1: No roles, no scope - return all allowed grants unchanged
	t.Run("no roles, no scope - return all", func(t *testing.T) {
		allowed := []string{"root", "admin", "operator"}
		roles := []Role{}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		expected := NewGrants("root", "admin", "operator")
		require.Equal(t, expected, result)
	})

	// Test case 2: No roles, no scope with scoped grants
	t.Run("no roles, no scope - with scoped grants", func(t *testing.T) {
		allowed := []string{"admin:ns1", "admin:ns2", "guest:ns3"}
		roles := []Role{}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		expected := NewGrants("admin:ns1", "admin:ns2", "guest:ns3")
		require.Equal(t, expected, result)
	})

	// Test case 3: Roles specified, scope specified - root in allowed, admin role with scope
	// root allows any role:scope combination
	t.Run("root in allowed, admin role, ns2 scope", func(t *testing.T) {
		allowed := []string{"root"}
		roles := []Role{RoleAdmin}
		scope := "ns2"

		result := FilterGrantStrings(allowed, roles, scope)
		expected := NewGrants("admin:ns2")
		require.Equal(t, expected, result)
	})

	// Test case 4: admin in allowed (no scope), admin role, ns2 scope
	// admin without scope does not grant admin:ns2
	t.Run("admin in allowed, admin role, ns2 scope", func(t *testing.T) {
		allowed := []string{"admin"}
		roles := []Role{RoleAdmin}
		scope := "ns2"

		result := FilterGrantStrings(allowed, roles, scope)
		// admin grant without scope doesn't match admin:ns2, and there's no admin:ns2 or root
		require.Empty(t, result)
	})

	// Test case 5: admin in allowed (no scope), guest role, ns2 scope
	// guest:ns2 is allowed because admin without scope allows guest on any scope
	t.Run("admin in allowed, guest role, ns2 scope", func(t *testing.T) {
		allowed := []string{"admin"}
		roles := []Role{RoleGuest}
		scope := "ns2"

		result := FilterGrantStrings(allowed, roles, scope)
		// admin without scope means HasRoleOn("", RoleAdmin) is true
		// For guest role with scope, FilterGrant checks:
		// - HasRoleOn("ns2", RoleOperator, RoleAdmin) -> false (no admin:ns2)
		// - HasRoleOn("", RoleOperator, RoleAdmin) -> true (has admin)
		// So guest:ns2 is granted
		expected := NewGrants("guest:ns2")
		require.Equal(t, expected, result)
	})

	// Test case 6: admin:ns1,admin:ns2,guest:ns3 in allowed, admin role, ns2 scope
	t.Run("multiple scoped grants, admin role, ns2 scope", func(t *testing.T) {
		allowed := []string{"admin:ns1", "admin:ns2", "guest:ns3"}
		roles := []Role{RoleAdmin}
		scope := "ns2"

		result := FilterGrantStrings(allowed, roles, scope)
		// HasGrant("admin:ns2") is true, so it returns admin:ns2
		expected := NewGrants("admin:ns2")
		require.Equal(t, expected, result)
	})

	// Test case 7: admin:ns1,admin:ns2,guest:ns3 in allowed, admin role, ns3 scope
	t.Run("multiple scoped grants, admin role, ns3 scope - no match", func(t *testing.T) {
		allowed := []string{"admin:ns1", "admin:ns2", "guest:ns3"}
		roles := []Role{RoleAdmin}
		scope := "ns3"

		result := FilterGrantStrings(allowed, roles, scope)
		// No admin:ns3 grant exists, and no admin without scope, and no root
		require.Empty(t, result)
	})

	// Test case 8: admin:ns1,admin:ns2,guest:ns3 in allowed, guest role, ns2 scope
	t.Run("multiple scoped grants, guest role, ns2 scope", func(t *testing.T) {
		allowed := []string{"admin:ns1", "admin:ns2", "guest:ns3"}
		roles := []Role{RoleGuest}
		scope := "ns2"

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterGrant("guest:ns2"):
		// - HasRole(Root) -> false
		// - HasGrant("guest:ns2") -> false
		// - role is RoleGuest:
		//   - HasRoleOn("ns2", RoleOperator, RoleAdmin) -> true (admin:ns2 exists)
		// So guest:ns2 is granted
		expected := NewGrants("guest:ns2")
		require.Equal(t, expected, result)
	})

	// Test case 9: Roles specified, no scope - root role
	t.Run("root role, no scope", func(t *testing.T) {
		allowed := []string{"root"}
		roles := []Role{RoleRoot}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterRole(RoleRoot):
		// - HasRole(RoleRoot) -> true
		// - returns root
		expected := NewGrants("root")
		require.Equal(t, expected, result)
	})

	// Test case 10: admin role, no scope
	t.Run("admin role, no scope", func(t *testing.T) {
		allowed := []string{"admin"}
		roles := []Role{RoleAdmin}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterRole(RoleAdmin):
		// - HasRole(RoleRoot) -> false
		// - role is RoleAdmin:
		//   - HasRoleOn("", RoleAdmin) -> true (has admin without scope)
		// - returns admin
		expected := NewGrants("admin")
		require.Equal(t, expected, result)
	})

	// Test case 11: guest role, no scope, with admin grants
	t.Run("guest role, no scope, with admin grants", func(t *testing.T) {
		allowed := []string{"admin"}
		roles := []Role{RoleGuest}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterRole(RoleGuest):
		// - HasRole(RoleRoot) -> false
		// - role is RoleGuest:
		//   - HasRoleOn("", RoleOperator, RoleAdmin, RoleGuest) -> true (has admin)
		// - returns guest
		expected := NewGrants("guest")
		require.Equal(t, expected, result)
	})

	// Test case 12: root role, no scope, with admin grants
	t.Run("root role, no scope, with admin grants", func(t *testing.T) {
		allowed := []string{"admin"}
		roles := []Role{RoleRoot}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterRole(RoleRoot):
		// - HasRole(RoleRoot) -> false
		// - role is RoleRoot (not in switch for admin/operator/guest)
		// - HasGrant("root") -> false
		// - returns empty
		require.Empty(t, result)
	})

	// Test case 13: admin role, no scope, with scoped grants
	t.Run("admin role, no scope, with scoped grants", func(t *testing.T) {
		allowed := []string{"admin:ns1", "admin:ns2", "guest:ns3"}
		roles := []Role{RoleAdmin}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterRole(RoleAdmin):
		// - HasRole(RoleRoot) -> false
		// - role is RoleAdmin:
		//   - HasRoleOn("", RoleAdmin) -> false (no admin without scope)
		//   - Namespaces(RoleAdmin) -> ["ns1", "ns2"]
		// - returns admin:ns1, admin:ns2 (order may vary)
		expected := NewGrants("admin:ns1", "admin:ns2")
		require.Equal(t, len(expected), len(result))
		require.ElementsMatch(t, []Grant(expected), []Grant(result))
	})

	// Test case 14: guest role, no scope, with scoped grants
	t.Run("guest role, no scope, with scoped grants", func(t *testing.T) {
		allowed := []string{"admin:ns1", "admin:ns2", "guest:ns3"}
		roles := []Role{RoleGuest}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterRole(RoleGuest):
		// - HasRole(RoleRoot) -> false
		// - role is RoleGuest:
		//   - HasRoleOn("", RoleOperator, RoleAdmin, RoleGuest) -> false
		//   - Namespaces(RoleAdmin, RoleOperator, RoleGuest) -> ["ns1", "ns2", "ns3"]
		// - returns guest:ns1, guest:ns2, guest:ns3 (order may vary)
		expected := NewGrants("guest:ns1", "guest:ns2", "guest:ns3")
		require.Equal(t, len(expected), len(result))
		require.ElementsMatch(t, []Grant(expected), []Grant(result))
	})

	// Test case 15: root role, no scope, with scoped grants
	t.Run("root role, no scope, with scoped grants", func(t *testing.T) {
		allowed := []string{"admin:ns1", "admin:ns2", "guest:ns3"}
		roles := []Role{RoleRoot}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterRole(RoleRoot):
		// - HasRole(RoleRoot) -> false
		// - role is RoleRoot (not in switch)
		// - HasGrant("root") -> false
		// - returns empty
		require.Empty(t, result)
	})

	// Test case 16: No roles, scope specified - root grant
	t.Run("no roles, ns1 scope, with root grant", func(t *testing.T) {
		allowed := []string{"root"}
		roles := []Role{}
		scope := "ns1"

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterScope("ns1"):
		// - HasRole(RoleRoot) -> true (root grant exists)
		// - returns admin:ns1
		expected := NewGrants("admin:ns1")
		require.Equal(t, expected, result)
	})

	// Test case 17: No roles, scope specified - guest grant
	t.Run("no roles, ns1 scope, with guest grant", func(t *testing.T) {
		allowed := []string{"guest"}
		roles := []Role{}
		scope := "ns1"

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterScope("ns1"):
		// - HasRole(RoleRoot) -> false
		// - Has(RoleAdmin, "ns1") -> false
		// - Has(RoleAdmin, "") -> false
		// - Has(RoleOperator, "ns1") -> false
		// - Has(RoleOperator, "") -> false
		// - Has(RoleGuest, "ns1") -> false
		// - Has(RoleGuest, "") -> true (guest grant without scope)
		// - returns guest:ns1
		expected := NewGrants("guest:ns1")
		require.Equal(t, expected, result)
	})

	// Test case 18: No roles, ns1 scope, with multiple scoped grants
	t.Run("no roles, ns1 scope, with multiple scoped grants", func(t *testing.T) {
		allowed := []string{"admin:ns1", "admin:ns2", "guest:ns3"}
		roles := []Role{}
		scope := "ns1"

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterScope("ns1"):
		// - HasRole(RoleRoot) -> false
		// - Has(RoleAdmin, "ns1") -> true
		// - returns admin:ns1
		expected := NewGrants("admin:ns1")
		require.Equal(t, expected, result)
	})

	// Test case 19: No roles, ns3 scope, with multiple scoped grants
	t.Run("no roles, ns3 scope, with multiple scoped grants", func(t *testing.T) {
		allowed := []string{"admin:ns1", "admin:ns2", "guest:ns3"}
		roles := []Role{}
		scope := "ns3"

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterScope("ns3"):
		// - HasRole(RoleRoot) -> false
		// - Has(RoleAdmin, "ns3") -> false
		// - Has(RoleAdmin, "") -> false
		// - Has(RoleOperator, "ns3") -> false
		// - Has(RoleOperator, "") -> false
		// - Has(RoleGuest, "ns3") -> true
		// - returns guest:ns3
		expected := NewGrants("guest:ns3")
		require.Equal(t, expected, result)
	})

	// Test case 20: Multiple roles, no scope
	t.Run("multiple roles, no scope", func(t *testing.T) {
		allowed := []string{"admin:ns1", "operator:ns1", "guest:ns2"}
		roles := []Role{RoleAdmin, RoleGuest}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterRole(RoleAdmin) -> admin:ns1
		// FilterRole(RoleGuest) -> guest:ns1, guest:ns2 (all namespaces from admin, operator, guest)
		// Result: admin:ns1, guest:ns2, guest:ns1 (order may vary)
		expected := NewGrants("admin:ns1", "guest:ns1", "guest:ns2")
		require.Equal(t, len(expected), len(result))
		require.ElementsMatch(t, []Grant(expected), []Grant(result))
	})

	// Test case 21: Multiple roles, with scope
	t.Run("multiple roles, with scope", func(t *testing.T) {
		allowed := []string{"admin:ns1", "operator:ns1", "guest:ns2"}
		roles := []Role{RoleAdmin, RoleGuest}
		scope := "ns1"

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterGrant("admin:ns1") -> admin:ns1 (HasGrant is true)
		// FilterGrant("guest:ns1") -> guest:ns1 (HasRoleOn("ns1", RoleOperator, RoleAdmin) is true because admin:ns1 exists)
		// Result: admin:ns1, guest:ns1
		expected := NewGrants("admin:ns1", "guest:ns1")
		require.Equal(t, expected, result)
	})

	// Test case 22: root grant, no roles, no scope
	t.Run("root grant, no roles, no scope", func(t *testing.T) {
		allowed := []string{"root"}
		roles := []Role{}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		expected := NewGrants("root")
		require.Equal(t, expected, result)
	})

	// Test case 23: Empty allowed grants
	t.Run("empty allowed grants", func(t *testing.T) {
		allowed := []string{}
		roles := []Role{RoleAdmin}
		scope := "ns1"

		result := FilterGrantStrings(allowed, roles, scope)
		require.Empty(t, result)
	})

	// Test case 24: root in allowed, requesting admin role with scope
	t.Run("root in allowed, requesting admin role with scope", func(t *testing.T) {
		allowed := []string{"root"}
		roles := []Role{RoleAdmin}
		scope := "ns1"

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterGrant("admin:ns1"):
		// - HasRole(RoleRoot) -> true (root grant exists)
		// - returns admin:ns1
		expected := NewGrants("admin:ns1")
		require.Equal(t, expected, result)
	})

	// Test case 25: Operator role with scope, admin in allowed
	t.Run("operator role with scope, admin in allowed", func(t *testing.T) {
		allowed := []string{"admin"}
		roles := []Role{RoleOperator}
		scope := "ns1"

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterGrant("operator:ns1"):
		// - HasRole(RoleRoot) -> false
		// - HasGrant("operator:ns1") -> false
		// - role is RoleOperator:
		//   - HasRoleOn("ns1", RoleAdmin) -> false
		//   - HasRoleOn("", RoleAdmin) -> true (has admin without scope)
		// - returns operator:ns1
		expected := NewGrants("operator:ns1")
		require.Equal(t, expected, result)
	})

	// Test case 26: Prioritizer role (non-scoped role), no scope
	t.Run("prioritizer role, no scope", func(t *testing.T) {
		allowed := []string{"prioritizer"}
		roles := []Role{RolePrioritizer}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterRole(RolePrioritizer):
		// - HasRole(RoleRoot) -> false
		// - role is not RoleAdmin, RoleOperator, RoleGuest
		// - HasGrant("prioritizer") -> true
		// - returns prioritizer
		expected := NewGrants("prioritizer")
		require.Equal(t, expected, result)
	})

	// Test case 27: Prioritizer role with scope
	t.Run("prioritizer role with scope", func(t *testing.T) {
		allowed := []string{"prioritizer"}
		roles := []Role{RolePrioritizer}
		scope := "ns1"

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterGrant("prioritizer:ns1"):
		// - HasRole(RoleRoot) -> false
		// - HasGrant("prioritizer:ns1") -> false
		// - role is RolePrioritizer (not in guest/operator checks)
		// - returns empty
		require.Empty(t, result)
	})

	// Test case 28: Admin with scope, requesting same admin role with same scope
	t.Run("admin:ns1 in allowed, requesting admin role with ns1 scope", func(t *testing.T) {
		allowed := []string{"admin:ns1"}
		roles := []Role{RoleAdmin}
		scope := "ns1"

		result := FilterGrantStrings(allowed, roles, scope)
		expected := NewGrants("admin:ns1")
		require.Equal(t, expected, result)
	})

	// Test case 29: Admin with scope, requesting admin role with different scope
	t.Run("admin:ns1 in allowed, requesting admin role with ns2 scope", func(t *testing.T) {
		allowed := []string{"admin:ns1"}
		roles := []Role{RoleAdmin}
		scope := "ns2"

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterGrant("admin:ns2"):
		// - HasRole(RoleRoot) -> false
		// - HasGrant("admin:ns2") -> false
		// - role is RoleAdmin, no admin:ns2 or admin without scope
		// - returns empty
		require.Empty(t, result)
	})

	// Test case 30: Admin without scope, requesting operator role without scope
	t.Run("admin in allowed, requesting operator role without scope", func(t *testing.T) {
		allowed := []string{"admin"}
		roles := []Role{RoleOperator}
		scope := ""

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterRole(RoleOperator):
		// - HasRole(RoleRoot) -> false
		// - role is RoleOperator:
		//   - HasRoleOn("", RoleAdmin, RoleOperator) -> true (has admin without scope)
		// - returns operator
		expected := NewGrants("operator")
		require.Equal(t, expected, result)
	})

	// Test case 31: Operator with scope, requesting admin role with same scope
	t.Run("operator:ns1 in allowed, requesting admin role with ns1 scope", func(t *testing.T) {
		allowed := []string{"operator:ns1"}
		roles := []Role{RoleAdmin}
		scope := "ns1"

		result := FilterGrantStrings(allowed, roles, scope)
		// FilterGrant("admin:ns1"):
		// - HasRole(RoleRoot) -> false
		// - HasGrant("admin:ns1") -> false
		// - role is RoleAdmin, no admin:ns1 or admin without scope
		// - returns empty
		require.Empty(t, result)
	})
}
