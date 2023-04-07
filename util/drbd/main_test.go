package drbd

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDump(t *testing.T) {
	b, err := os.ReadFile(path.Join("testdata", "drbdadm.dump-xml"))
	require.Nil(t, err)

	t.Run("digest", func(t *testing.T) {
		expectedMinors := map[string]any{
			"0": nil,
			"1": nil,
			"2": nil,
			"3": nil,
			"4": nil,
			"5": nil,
		}
		expectedPorts := map[string]any{
			"7289": nil,
			"7290": nil,
			"7291": nil,
			"7292": nil,
			"7293": nil,
			"7294": nil,
		}
		digest, err := ParseDigest(b)
		require.Nil(t, err)
		require.Equal(t, expectedMinors, digest.Minors)
		require.Equal(t, expectedPorts, digest.Ports)
	})
	t.Run("parse dump", func(t *testing.T) {
		dump, err := ParseConfig(b)
		require.Nil(t, err)
		resource, ok := dump.GetResource("demo-focal1")
		require.True(t, ok)
		host, ok := resource.GetHost("magnetar")
		require.True(t, ok)
		require.Equal(t, "54.37.191.100", host.Address.IP)
		require.Equal(t, "7292", host.Address.Port)
		volume, ok := host.GetVolume("0")
		require.True(t, ok)
		require.Equal(t, "/dev/drbd3", volume.Device.Path)
		require.Equal(t, "internal", volume.MetaDisk)
		require.Equal(t, "3", volume.Device.Minor)
		require.Equal(t, "/dev/demo-focal/focal1", volume.Disk)
	})
}
