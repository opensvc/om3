package daemonapi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/daemon/api"
)

func ptr(s string) *string {
	return &s
}

func TestNodeRegisterCredentialError(t *testing.T) {
	t.Run("accepts both halves set", func(t *testing.T) {
		payload := api.PostNodeActionRegisterRequest{User: ptr("collectoruser"), Password: ptr("s3cret")}
		assert.NoError(t, nodeRegisterCredentialError(payload))
	})

	t.Run("accepts neither half set", func(t *testing.T) {
		// The node then registers with the id it already holds.
		assert.NoError(t, nodeRegisterCredentialError(api.PostNodeActionRegisterRequest{}))
		payload := api.PostNodeActionRegisterRequest{User: ptr(""), Password: ptr("")}
		assert.NoError(t, nodeRegisterCredentialError(payload))
	})

	t.Run("refuses a user without a password", func(t *testing.T) {
		// It would reach the password prompt of the register command, on a
		// daemon that has no terminal to prompt on.
		payload := api.PostNodeActionRegisterRequest{User: ptr("collectoruser")}
		require.Error(t, nodeRegisterCredentialError(payload))
	})

	t.Run("refuses a password without a user", func(t *testing.T) {
		// It names nobody to authenticate as, and would be dropped: the node
		// would register with the id it already holds, which is not what the
		// caller asked for.
		payload := api.PostNodeActionRegisterRequest{Password: ptr("s3cret")}
		require.Error(t, nodeRegisterCredentialError(payload))
	})
}

func TestNodeRegisterArgs(t *testing.T) {
	t.Run("names the app when the body has one", func(t *testing.T) {
		payload := api.PostNodeActionRegisterRequest{App: ptr("MyApp")}
		assert.Equal(t, []string{"node", "register", "--app", "MyApp"}, nodeRegisterArgs(payload))
	})

	t.Run("leaves the app out when the body has none", func(t *testing.T) {
		assert.Equal(t, []string{"node", "register"}, nodeRegisterArgs(api.PostNodeActionRegisterRequest{}))
	})

	t.Run("never carries the credentials", func(t *testing.T) {
		// A command line is readable by any user through /proc/<pid>/cmdline,
		// and the command string is published on the bus and kept in the exec
		// store.
		payload := api.PostNodeActionRegisterRequest{
			User:     ptr("collectoruser"),
			Password: ptr("s3cret"),
			App:      ptr("MyApp"),
		}
		assert.NotContains(t, strings.Join(nodeRegisterArgs(payload), " "), "s3cret")
		assert.NotContains(t, strings.Join(nodeRegisterArgs(payload), " "), "collectoruser")
	})
}

func TestNodeRegisterVarEnv(t *testing.T) {
	t.Run("hands the credentials to the forked command", func(t *testing.T) {
		payload := api.PostNodeActionRegisterRequest{User: ptr("collectoruser"), Password: ptr("s3cret")}
		want := []string{env.CollectorCredentialVar + "=collectoruser:s3cret"}
		assert.Equal(t, want, nodeRegisterVarEnv(payload))
	})

	t.Run("adds nothing without a user", func(t *testing.T) {
		assert.Empty(t, nodeRegisterVarEnv(api.PostNodeActionRegisterRequest{}))
	})
}
