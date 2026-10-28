package object

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

// The status says the checksum of the configuration it was computed from,
// as the daemon computes it of the file: the md5 of its bytes.
func TestStatusSaysTheChecksumOfItsConfig(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "s1", Kind: naming.KindSvc, Namespace: "ns1"}
	b := []byte("[DEFAULT]\nid = 6c9c2bd5-3a41-4a0c-a6d4-3b3f1f0a2e11\n")
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), b, 0644))

	o, err := NewSvc(p)
	require.NoError(t, err)
	st, err := o.FreshStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%x", md5.Sum(b)), st.ConfigCsum)
}
