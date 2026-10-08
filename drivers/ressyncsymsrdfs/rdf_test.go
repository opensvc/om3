package ressyncsymsrdfs

import (
	"encoding/xml"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The wwn map the sync writes pairs each device with its remote half, which
// the symmetrix driver reads from the RDF element of the device.
func TestDeviceListGivesTheRemoteWWN(t *testing.T) {
	b, err := os.ReadFile("../arraysymmetrix/testdata/06-symdev_show_wwn")
	require.NoError(t, err)
	var data XDevList
	require.NoError(t, xml.Unmarshal(b, &data))
	require.NotEmpty(t, data.Symmetrix.Devices)
	var found bool
	for _, device := range data.Symmetrix.Devices {
		if device.RDF == nil {
			continue
		}
		found = true
		assert.Equal(t, "60000000000000000000000000000005", device.RDF.RemoteWWN())
	}
	assert.True(t, found, "no paired device in the testdata")
}
