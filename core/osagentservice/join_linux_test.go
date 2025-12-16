package osagentservice

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"
)

func TestJoin(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	env := testhelper.Setup(t)
	rawconfig.Load(map[string]string{
		"OSVC_ROOT_PATH":    env.Root,
		"OSVC_CLUSTER_NAME": env.ClusterName,
	})
	err := capabilities.Scan(t.Context())
	require.NoError(t, err)
	err = Join()
	require.ErrorIs(t, err, os.ErrNotExist)
}
