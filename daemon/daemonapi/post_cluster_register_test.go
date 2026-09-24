package daemonapi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/daemon/api"
)

func TestClusterRegisterArgs(t *testing.T) {
	app := "MyApp"
	empty := ""
	credential := "collectoruser:s3cret"

	t.Run("names the app when the body has one", func(t *testing.T) {
		args := clusterRegisterArgs(api.ClusterRegisterBody{App: &app})
		assert.Equal(t, []string{"cluster", "register", "--app", "MyApp"}, args)
	})

	t.Run("leaves the app out when the body has none", func(t *testing.T) {
		assert.Equal(t, []string{"cluster", "register"}, clusterRegisterArgs(api.ClusterRegisterBody{}))
		assert.Equal(t, []string{"cluster", "register"}, clusterRegisterArgs(api.ClusterRegisterBody{App: &empty}))
	})

	t.Run("never carries the credential", func(t *testing.T) {
		// A command line is readable by any user through /proc/<pid>/cmdline,
		// so the credential travels in the environment instead.
		args := clusterRegisterArgs(api.ClusterRegisterBody{App: &app, Credential: &credential})
		assert.NotContains(t, strings.Join(args, " "), "s3cret")
	})
}

func TestClusterRegisterEnviron(t *testing.T) {
	t.Run("hands the credential to the forked command", func(t *testing.T) {
		environ := clusterRegisterEnviron([]string{"PATH=/bin"}, "collectoruser:s3cret")
		assert.Equal(t, []string{"PATH=/bin", env.CollectorCredentialVar + "=collectoruser:s3cret"}, environ)
	})

	t.Run("adds nothing without a credential", func(t *testing.T) {
		// Every node then registers with the id it already holds.
		environ := clusterRegisterEnviron([]string{"PATH=/bin"}, "")
		assert.Equal(t, []string{"PATH=/bin"}, environ)
	})
}
