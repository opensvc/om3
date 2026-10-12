//go:build linux

package resipcni

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A network names a cni configuration of the configuration directory: one
// naming a path elsewhere is refused before anything is read, as its type
// would name the plugin run as root.
func TestANetworkNamingAPathIsRefused(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "evil.conf"), []byte(`{"type": "bridge"}`), 0644))
	rel, err := filepath.Rel(dir, filepath.Join(outside, "evil"))
	require.NoError(t, err)
	for _, name := range []string{rel, filepath.Join(outside, "evil"), "..", "."} {
		r := &T{CNIConfig: dir, Network: name}
		assert.Error(t, r.Configure(), name)
		_, err := r.netConfBytes()
		assert.Error(t, err, name)
	}
}

// A plugin is a file of the plugin directories, which a type naming a path
// is not.
func TestAPluginTypeNamingAPathIsNone(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bridge"), []byte{}, 0755))
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0755))
	r := &T{CNIPlugins: sub}
	assert.Equal(t, "", r.pluginFile("../bridge"))
	assert.Equal(t, "", r.pluginFile(filepath.Join(dir, "bridge")))
	r.CNIPlugins = dir
	assert.Equal(t, filepath.Join(dir, "bridge"), r.pluginFile("bridge"))
}
