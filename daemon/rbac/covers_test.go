package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A grantor gives what it holds, and nothing more.
func TestCovers(t *testing.T) {
	for _, tc := range []struct {
		holds  Grants
		g      Grant
		covers bool
	}{
		{Grants{GrantRoot}, "root", true},
		{Grants{GrantRoot}, "admin:ns1", true},
		{Grants{GrantRoot}, "squatter", true},
		{NewGrants("admin:system"), "root", false},
		{NewGrants("admin:system"), "admin:system", true},
		{NewGrants("admin:system"), "admin:ns1", false},
		{NewGrants("admin:system"), "guest:system", false},
		{NewGrants("admin"), "admin:ns1", true},
		{NewGrants("admin"), "squatter", false},
		{NewGrants("squatter"), "squatter", true},
		{NewGrants("guest:ns1", "operator:ns1"), "operator:ns1", true},
	} {
		assert.Equalf(t, tc.covers, tc.holds.Covers(tc.g), "%s covers %s", tc.holds, tc.g)
	}
	assert.Equal(t, Grants{"root"}, NewGrants("admin:system").Uncovered("admin:system", "root"))
	assert.Empty(t, NewGrants("admin:system").Uncovered())
}
