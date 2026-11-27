package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/rbac"
)

// TestDNSZoneSees pins who reads the dump of the zone, and the records of
// which namespaces: root and the inspector every one's, a reader of every
// namespace too, a reader of some the ones of its namespaces, and a user
// reading none, as a heartbeat or a guest of nothing, none.
func TestDNSZoneSees(t *testing.T) {
	for name, grants := range map[string]rbac.Grants{
		"root":                     {rbac.GrantRoot},
		"inspector":                {rbac.GrantInspector},
		"guest of every namespace": {rbac.NewGrant(rbac.RoleGuest, "")},
	} {
		sees, ok := dnsZoneSees(grants)
		require.True(t, ok, name)
		assert.Nil(t, sees, "%s sees every namespace", name)
	}
	for _, role := range []rbac.Role{rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin} {
		sees, ok := dnsZoneSees(rbac.Grants{rbac.NewGrant(role, "prod"), rbac.NewGrant(role, "test*")})
		require.True(t, ok, role)
		require.NotNil(t, sees, role)
		assert.True(t, sees("prod"), role)
		assert.True(t, sees("test2"), "%s of a pattern", role)
		assert.False(t, sees("dev"), role)
		assert.False(t, sees("root"), role)
	}
	for name, grants := range map[string]rbac.Grants{
		"none":      {},
		"heartbeat": {rbac.GrantHeartbeat},
		"scraper":   {rbac.GrantScraper},
	} {
		_, ok := dnsZoneSees(grants)
		assert.False(t, ok, name)
	}
}
