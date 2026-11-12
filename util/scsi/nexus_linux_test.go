//go:build linux

package scsi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pg83 is the device identification page of a lio lun reached by its
// relative target port 0xa: a naa designator of the lun, then the relative
// target port designator, associated with the target port.
var pg83 = []byte{
	0x00, 0x83, 0x00, 0x1c,
	0x01, 0x03, 0x00, 0x10, 0x60, 0x01, 0x40, 0x55, 0x0b, 0x8e, 0x30, 0xad, 0xba, 0x14, 0x2d, 0x5b, 0x5f, 0xee, 0x01, 0x9f,
	0x51, 0x14, 0x00, 0x04, 0x00, 0x00, 0x00, 0x0a,
}

func TestRelativeTargetPort(t *testing.T) {
	port, ok := relativeTargetPort(pg83)
	assert.True(t, ok)
	assert.Equal(t, 0xa, port)
	_, ok = relativeTargetPort(pg83[:24])
	assert.False(t, ok, "no relative target port designator")
}

// The nexus of a path is the initiator name and the session id of its iscsi
// session, and the relative target port of its identification page.
func TestPathNexus(t *testing.T) {
	root := t.TempDir()
	saved := sysRoot
	sysRoot = root
	t.Cleanup(func() { sysRoot = saved })

	dev := filepath.Join(root, "devices/platform/host7/session2/target7:0:0/7:0:0:10")
	require.NoError(t, os.MkdirAll(dev, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dev, "vpd_pg83"), pg83, 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "block/sda"), 0o755))
	require.NoError(t, os.Symlink(dev, filepath.Join(root, "block/sda/device")))
	session := filepath.Join(root, "class/iscsi_session/session2")
	require.NoError(t, os.MkdirAll(session, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(session, "initiatorname"), []byte("iqn.2009-11.com.opensvc.srv:dev2n1.storage.initiator\n"), 0o644))

	n, err := pathNexus("sda")
	require.NoError(t, err)
	assert.Equal(t, nexus{TransportID: "iqn.2009-11.com.opensvc.srv:dev2n1.storage.initiator,i,0x00023d000002", RelativeTargetPort: 0xa}, n)
	assert.True(t, parseFullStatus([]byte(sgFullStatus))[0].isCurrent([]nexus{n}), "the registration of session2 by port 0xa is the one of the path")

	fc := filepath.Join(root, "devices/pci/host3/rport-3:0-0/target3:0:0/3:0:0:1")
	require.NoError(t, os.MkdirAll(fc, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "block/sdz"), 0o755))
	require.NoError(t, os.Symlink(fc, filepath.Join(root, "block/sdz/device")))
	_, err = pathNexus("sdz")
	assert.ErrorIs(t, err, ErrNoNexus, "a fibre channel path, whose nexus a restart does not change")
}

// The nexus of the iscsi paths of the node, read from its sysfs, when there
// are: a read-only check of the matching against a real lun.
func TestPathNexusOfThisNode(t *testing.T) {
	for _, name := range []string{"sda", "sdb"} {
		if _, err := os.Stat(filepath.Join("/sys/block", name, "device/vpd_pg83")); err != nil {
			t.Skip("no scsi disk")
		}
		n, err := pathNexus(name)
		if err != nil {
			t.Skipf("%s: %s", name, err)
		}
		t.Logf("%s: %+v", name, n)
	}
}
