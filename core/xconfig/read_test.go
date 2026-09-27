package xconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/key"
)

// A read never changes the configuration. The ini file makes a section, and a
// key, on the first access to one it does not have, and the next commit wrote
// what a read had made: an empty [node] section appeared in configurations
// whose resources read a node keyword, at the first unrelated write.
func TestAReadDoesNotChangeTheConfiguration(t *testing.T) {
	cfg, err := NewObject("", []byte("[fs#0]\ntype = tmpfs\n"))
	require.NoError(t, err)

	absent := key.T{Section: "node", Option: "prkey"}
	assert.False(t, cfg.HasKey(absent))
	assert.Empty(t, cfg.Get(absent))
	_, err = cfg.GetStrict(absent)
	assert.Error(t, err)
	assert.Empty(t, cfg.Keys("node"))

	missingKey := key.T{Section: "fs#0", Option: "size"}
	assert.Empty(t, cfg.Get(missingKey))
	assert.False(t, cfg.HasKey(missingKey), "reading a key does not make it")

	assert.NotContains(t, cfg.SectionStrings(), "node", "reading a section does not make it")

	v, err := cfg.GetStrict(key.T{Section: "fs#0", Option: "type"})
	require.NoError(t, err)
	assert.Equal(t, "tmpfs", v, "what exists is still read")
}
