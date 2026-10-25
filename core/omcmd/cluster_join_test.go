package omcmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/key"
)

// TestCollectorRegistrationSurvivesTheJoin pins that the registration id
// read before the join moves the node config aside, and the one stored after
// it in the fresh node config, are the node.uuid keyword ones.
func TestCollectorRegistrationSurvivesTheJoin(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte("[node]\nuuid = uuid-old\n"), 0600))
	assert.Equal(t, "uuid-old", localNodeUUID())

	// The join moves the node config aside.
	require.NoError(t, os.Remove(rawconfig.NodeConfigFile()))
	assert.Empty(t, localNodeUUID())

	require.NoError(t, storeCollectorRegistration("uuid-new"))
	n, err := object.NewNode()
	require.NoError(t, err)
	assert.Equal(t, "uuid-new", n.Config().GetString(key.Parse("node.uuid")))
}
