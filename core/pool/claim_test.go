package pool

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

func writeNamespaceConfig(t *testing.T, namespace, text string) {
	t.Helper()
	p := naming.Path{Namespace: namespace, Kind: naming.KindNscfg, Name: "namespace"}
	configFile := p.ConfigFile()
	require.NoError(t, os.MkdirAll(filepath.Dir(configFile), os.ModePerm))
	require.NoError(t, os.WriteFile(configFile, []byte(text), 0644))
}

func TestClaimLimit(t *testing.T) {
	testhelper.Setup(t)

	writeNamespaceConfig(t, "capped", `
[claim#1]
type = pool
name = dirquota
limit = 250m
`)

	t.Run("the limit of a pool claim is a size", func(t *testing.T) {
		limit, capped, err := ClaimLimit("capped", "dirquota")
		require.NoError(t, err)
		assert.True(t, capped)
		assert.Equal(t, int64(250*1024*1024), limit)
	})
	t.Run("a namespace with no claim on the pool is not capped", func(t *testing.T) {
		_, capped, err := ClaimLimit("capped", "other")
		require.NoError(t, err)
		assert.False(t, capped)
	})
}

// TestClaimFitsUncappedNeedsNoDaemon verifies the common case, a namespace
// claiming nothing of the pool, is answered from the local configuration
// alone. The test runs with no daemon reachable, so a call to the api here
// would either fail or hang.
func TestClaimFitsUncappedNeedsNoDaemon(t *testing.T) {
	testhelper.Setup(t)

	writeNamespaceConfig(t, "uncapped", `
[DEFAULT]
env = TST
`)

	ok, why, err := ClaimFits(context.Background(), "uncapped", "dirquota", 1024*1024*1024)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "", why)
}
