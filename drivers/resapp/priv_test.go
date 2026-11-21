//go:build !windows

package resapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/pg"
)

// The privilege restrictions of the application are passed on to om exec,
// normalized, and not at all when the keywords are not set or false. A
// value that is not one fails the command, which is never run
// unrestricted, and is refused by the validation.
func TestToCapsPriv(t *testing.T) {
	newT := func() *T {
		r := &T{}
		r.SetPG(&pg.Config{})
		return r
	}
	caps, err := newT().toCaps(execCredential{})
	require.NoError(t, err)
	assert.Nil(t, caps.NoNewPrivileges)
	assert.Nil(t, caps.CapabilityBoundingSet)

	r := newT()
	r.NoNewPrivileges = "yes"
	r.CapabilityBoundingSet = "~cap_sys_admin"
	caps, err = r.toCaps(execCredential{})
	require.NoError(t, err)
	if assert.NotNil(t, caps.NoNewPrivileges) {
		assert.True(t, *caps.NoNewPrivileges)
	}
	if assert.NotNil(t, caps.CapabilityBoundingSet) {
		assert.Equal(t, "~CAP_SYS_ADMIN", *caps.CapabilityBoundingSet)
	}

	r = newT()
	r.NoNewPrivileges = "false"
	caps, err = r.toCaps(execCredential{})
	require.NoError(t, err)
	assert.Nil(t, caps.NoNewPrivileges)

	r = newT()
	r.NoNewPrivileges = "ture"
	_, err = r.toCaps(execCredential{})
	assert.ErrorContains(t, err, "no_new_privileges")
	r = newT()
	r.CapabilityBoundingSet = "CAP_BOGUS"
	_, err = r.toCaps(execCredential{})
	assert.ErrorContains(t, err, "unknown capabilities: CAP_BOGUS")

	assert.Error(t, validateBool("ture"))
	assert.NoError(t, validateBool("true"))
	assert.Error(t, validateCapabilityBoundingSet("CAP_BOGUS"))
	assert.NoError(t, validateCapabilityBoundingSet("~CAP_SYS_ADMIN"))
}
