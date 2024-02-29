package osagentservice

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/testhelper"
	"github.com/opensvc/om3/util/capabilities"
)

func TestJoin(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	env := testhelper.Setup(t)
	rawconfig.Load(map[string]string{
		"osvc_root_path":    env.Root,
		"osvc_cluster_name": env.ClusterName,
	})
	err := capabilities.Scan()
	require.NoError(t, err)
	err = Join()
	require.ErrorIs(t, err, os.ErrNotExist)
}
