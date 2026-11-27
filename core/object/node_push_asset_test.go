package object

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/testhelper"
)

// The machine uuid is node.machine_uuid or the probed one, never node.uuid,
// the collector authentication token of the node, a secret.
func TestNodeAssetMachineUUID(t *testing.T) {
	env := testhelper.Setup(t)
	token := "4b4c1c8e-0000-4000-8000-5ec2e7000000"
	write := func(s string) {
		require.NoError(t, os.WriteFile(filepath.Join(env.Root, "etc", "node.conf"), []byte(s), 0o600))
	}

	t.Run("never the collector token", func(t *testing.T) {
		write("[node]\nuuid = " + token + "\n")
		n, err := NewNode()
		require.NoError(t, err)
		data, err := n.getAsset()
		require.NoError(t, err)
		assert.NotEqual(t, token, data.Properties.MachineUUID.Value)
		assert.NotEqual(t, "config", data.Properties.MachineUUID.Source)
	})

	t.Run("node.machine_uuid overrides the probe", func(t *testing.T) {
		write("[node]\nuuid = " + token + "\nmachine_uuid = c5f98577-2918-4da7-b7c9-0ae24687a572\n")
		n, err := NewNode()
		require.NoError(t, err)
		data, err := n.getAsset()
		require.NoError(t, err)
		assert.Equal(t, "c5f98577-2918-4da7-b7c9-0ae24687a572", data.Properties.MachineUUID.Value)
	})
}
