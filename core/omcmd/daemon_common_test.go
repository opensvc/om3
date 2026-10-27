package omcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

// The paths kept and what they hold stay, the directories leading to them
// stay but their other entries, and the other entries go.
func TestRemoveTree(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{
		"backup/a",
		"namespaces/root/svc/foo/status.json",
		"pool/quota/dirquota1/data",
		"pool/other/x",
		"quota-pool.img",
	} {
		p = filepath.Join(dir, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0700))
		require.NoError(t, os.WriteFile(p, nil, 0600))
	}
	keep := []string{filepath.Join(dir, "backup"), filepath.Join(dir, "pool/quota")}
	require.NoError(t, removeTree(plog.NewDefaultLogger(), dir, keep))
	assert.FileExists(t, filepath.Join(dir, "backup/a"))
	assert.FileExists(t, filepath.Join(dir, "pool/quota/dirquota1/data"))
	assert.NoDirExists(t, filepath.Join(dir, "pool/other"))
	assert.NoDirExists(t, filepath.Join(dir, "namespaces"))
	assert.NoFileExists(t, filepath.Join(dir, "quota-pool.img"))
}
