package rescontainerkvm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/plog"
)

// fakeDumpxml puts a virsh in front of the PATH whose dumpxml prints xml,
// or fails when xml is empty.
func fakeDumpxml(t *testing.T, xml string) {
	dir := t.TempDir()
	def := filepath.Join(dir, "def.xml")
	require.NoError(t, os.WriteFile(def, []byte(xml), 0644))
	script := `#!/bin/sh
case "$1" in
dumpxml) [ -s ` + def + ` ] || { echo "error: failed to get domain" >&2; exit 1; }; cat ` + def + `;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "virsh"), []byte(script), 0755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func TestInfoUUID(t *testing.T) {
	r := &T{}
	r.Name = "k2"
	r.SetLoggerForTest(plog.NewDefaultLogger())

	t.Run("the uuid of the definition, lowercased", func(t *testing.T) {
		fakeDumpxml(t, "<domain type='kvm'>\n  <name>k2</name>\n  <uuid>C5F98577-2918-4DA7-B7C9-0AE24687A572</uuid>\n</domain>\n")
		keys, err := r.Info(context.Background())
		require.NoError(t, err)
		assert.Equal(t, resource.InfoKeys{{Key: "uuid", Value: "c5f98577-2918-4da7-b7c9-0ae24687a572"}}, keys)
	})
	t.Run("no key for a domain not defined", func(t *testing.T) {
		fakeDumpxml(t, "")
		keys, err := r.Info(context.Background())
		require.NoError(t, err)
		assert.Empty(t, keys)
	})
	t.Run("no key for a definition without uuid", func(t *testing.T) {
		fakeDumpxml(t, "<domain type='kvm'><name>k2</name></domain>\n")
		keys, err := r.Info(context.Background())
		require.NoError(t, err)
		assert.Empty(t, keys)
	})
}

func TestIsDomainNotFound(t *testing.T) {
	notFound := errors.New("virsh dumpxml --inactive k2: exit status 1: error: failed to get domain 'k2'")
	noDaemon := errors.New("virsh dumpxml --inactive k2: exit status 1: error: failed to connect to the hypervisor")
	assert.True(t, isDomainNotFound(notFound), "a domain not defined adds no key")
	assert.False(t, isDomainNotFound(noDaemon), "a failed lookup is returned, the cached uuid kept")
	assert.False(t, isDomainNotFound(nil))
}
