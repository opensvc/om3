package claim

import (
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

func TestLimit(t *testing.T) {
	testhelper.Setup(t)

	writeNamespaceConfig(t, "capped", `
[claim#1]
type = pool
name = dirquota
limit = 250m

[claim#2]
type = network
name = backend2
limit = 10

[claim#3]
type = pool
name = unlimited
`)
	writeNamespaceConfig(t, "uncapped", `
[DEFAULT]
env = TST
`)

	t.Run("a claim naming a limit caps the namespace on that resource", func(t *testing.T) {
		limit, capped, err := Limit("capped", "pool", "dirquota")
		require.NoError(t, err)
		assert.True(t, capped)
		assert.Equal(t, "250m", limit)
	})
	t.Run("a claim is found by type and name", func(t *testing.T) {
		limit, capped, err := Limit("capped", "network", "backend2")
		require.NoError(t, err)
		assert.True(t, capped)
		assert.Equal(t, "10", limit)
	})
	t.Run("a name claimed in another type does not match", func(t *testing.T) {
		_, capped, err := Limit("capped", "network", "dirquota")
		require.NoError(t, err)
		assert.False(t, capped)
	})
	t.Run("a claim naming no limit does not cap", func(t *testing.T) {
		_, capped, err := Limit("capped", "pool", "unlimited")
		require.NoError(t, err)
		assert.False(t, capped)
	})
	t.Run("a resource the namespace has no claim on does not cap", func(t *testing.T) {
		_, capped, err := Limit("capped", "pool", "other")
		require.NoError(t, err)
		assert.False(t, capped)
	})
	t.Run("a namespace claiming nothing does not cap", func(t *testing.T) {
		_, capped, err := Limit("uncapped", "pool", "dirquota")
		require.NoError(t, err)
		assert.False(t, capped)
	})
	t.Run("a namespace with no configuration here does not cap", func(t *testing.T) {
		_, capped, err := Limit("elsewhere", "pool", "dirquota")
		require.NoError(t, err)
		assert.False(t, capped)
	})
}
