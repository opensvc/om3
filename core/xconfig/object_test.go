package xconfig

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/util/key"
)

// TestSetIsKeywordOrderInsensitive asserts a DEFAULT section keyword lands in
// the DEFAULT section whatever its rank in the keyword list. Set after a
// resource keyword, "nodes" used to be written past the "[fs#1]" header, which
// reads back as a keyword of that resource.
func TestSetIsKeywordOrderInsensitive(t *testing.T) {
	for _, tc := range []struct {
		name     string
		config   string
		keywords []string
	}{
		{
			name:     "from scratch, default section keyword first",
			keywords: []string{"nodes=*", "fs#1.type=flag", "id=xxx"},
		},
		{
			name:     "from scratch, driver section keyword first",
			keywords: []string{"fs#1.type=flag", "nodes=*", "id=xxx"},
		},
		{
			name:     "on a config whose default section is empty",
			config:   "# about fs#1\n[fs#1]\ntype = flag\n",
			keywords: []string{"nodes=*", "id=xxx"},
		},
	} {
		for _, materialize := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s, materialized default section: %v", tc.name, materialize), func(t *testing.T) {
				p := filepath.Join(t.TempDir(), "svc1.conf")
				cfg, err := NewObject(p, []byte(tc.config))
				require.NoError(t, err)
				if materialize {
					cfg.MaterializeDefaultSection()
				}
				require.NoError(t, cfg.PrepareSet(keyop.ParseOps(tc.keywords)...))

				b, err := cfg.Dump()
				require.NoError(t, err)

				// Read the dump back: a keyword written after a section header
				// belongs to that section, whatever the in-memory state says.
				reloaded, err := NewObject(p, b)
				require.NoError(t, err)
				for _, expected := range []struct {
					key   string
					value string
				}{
					{"nodes", "*"},
					{"id", "xxx"},
					{"fs#1.type", "flag"},
					{"fs#1.nodes", ""},
					{"fs#1.id", ""},
				} {
					k := key.Parse(expected.key)
					require.Equalf(t, expected.value, reloaded.Get(k), "%s in\n%s", k, b)
				}
				if materialize {
					require.Truef(t, bytes.HasPrefix(b, []byte("[DEFAULT]\n")), "no default section header in\n%s", b)
				}
			})
		}
	}
}

// TestCreateShapedConfigLayout pins the layout the create codepaths produce: a
// "[DEFAULT]" header, and a blank line between sections, as OpenSVC v2 wrote
// them.
func TestCreateShapedConfigLayout(t *testing.T) {
	p := filepath.Join(t.TempDir(), "svc1.conf")
	cfg, err := NewObject(p, []byte(nil))
	require.NoError(t, err)
	cfg.MaterializeDefaultSection()
	keywords := []string{"fs#1.type=flag", "nodes=*", "app#1.type=simple", "id=xxx"}
	require.NoError(t, cfg.PrepareSet(keyop.ParseOps(keywords)...))

	b, err := cfg.Dump()
	require.NoError(t, err)
	require.Equal(t, `[DEFAULT]
nodes = *
id = xxx

[fs#1]
type = flag

[app#1]
type = simple
`, string(b))
}
