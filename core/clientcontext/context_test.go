package clientcontext

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The first cluster added on an account is saved, the configuration directory
// made for it.
func TestSaveMakesTheConfigDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	homedir.Reset()
	t.Cleanup(homedir.Reset)

	cfg, err := Load()
	require.NoError(t, err)
	require.NoError(t, cfg.AddCluster("c1", Cluster{Server: "https://n1:1215"}))
	require.NoError(t, cfg.Save())
	st, err := os.Stat(filepath.Join(home, ".config", "opensvc"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), st.Mode().Perm())

	cfg, err = Load()
	require.NoError(t, err)
	assert.Contains(t, cfg.Clusters, "c1")
}

// A context can be connected with when its cluster is defined, and its user,
// when it names one: a context naming no user logs in at the openid issuer of
// its cluster.
func TestSelectable(t *testing.T) {
	cfg := config{
		Clusters: map[string]Cluster{"c1": {Server: "https://n1:1215"}},
		Users:    map[string]User{"u1": {}},
	}
	assert.True(t, cfg.Selectable(Relation{ClusterRefName: "c1", UserRefName: "u1"}))
	assert.True(t, cfg.Selectable(Relation{ClusterRefName: "c1"}), "no user")
	assert.False(t, cfg.Selectable(Relation{ClusterRefName: "c1", UserRefName: "nobody"}), "a user not defined")
	assert.False(t, cfg.Selectable(Relation{ClusterRefName: "c2"}), "a cluster not defined")
}

// A context naming no user resolves, with no user name.
func TestNewWithoutUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	homedir.Reset()
	t.Cleanup(homedir.Reset)
	cfg, err := Load()
	require.NoError(t, err)
	require.NoError(t, cfg.AddCluster("c1", Cluster{Server: "https://n1:1215"}))
	require.NoError(t, cfg.AddContext("x", Relation{ClusterRefName: "c1"}))
	require.NoError(t, cfg.Save())

	t.Setenv("OSVC_CONTEXT", "x")
	c, err := New()
	require.NoError(t, err)
	assert.Equal(t, "https://n1:1215", c.Cluster.Server)
	assert.Equal(t, "", *c.User.Name)
}
