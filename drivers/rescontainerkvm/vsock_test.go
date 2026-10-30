//go:build linux

package rescontainerkvm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

func TestWithVsockDevice(t *testing.T) {
	args := []string{"/usr/bin/virt-install", "--name", "vm1"}
	got := withVsockDevice(args)
	assert.Equal(t, []string{"/usr/bin/virt-install", "--name", "vm1", "--vsock", "cid.auto=yes"}, got)
	assert.Len(t, args, 3, "the keyword value is left as it is")

	assert.True(t, isVirtInstall(args))
	assert.True(t, isVirtInstall([]string{"virt-install"}))
	assert.False(t, isVirtInstall([]string{"/opt/create-guest.sh"}), "a custom script")
	assert.False(t, isVirtInstall(nil))
}

func TestVirtInstWithVsockLeavesTheCommand(t *testing.T) {
	for name, r := range map[string]*T{
		"no port":        {VsockPort: 0, VirtInst: []string{"virt-install", "--name", "vm1"}},
		"custom script":  {VsockPort: 1215, VirtInst: []string{"/opt/create-guest.sh"}},
		"device already": {VsockPort: 1215, VirtInst: []string{"virt-install", "--vsock", "cid.address=5"}},
	} {
		assert.Equal(t, r.VirtInst, r.virtInstWithVsock(context.Background()), name)
	}
}

// The definition a node syncs to a hypervisor with no vsock support starts
// there from a copy without its device, otherwise the same.
func TestDropVsockDevice(t *testing.T) {
	const head = "<domain>\n  <devices>\n    <interface type='bridge'/>\n"
	const tail = "    <memballoon model='virtio'/>\n  </devices>\n</domain>\n"
	for name, device := range map[string]string{
		"allocated": "    <vsock model='virtio'>\n      <cid auto='yes'/>\n    </vsock>\n",
		"with pci":  "    <vsock model='virtio'>\n      <cid auto='yes' address='3'/>\n      <address type='pci' bus='0x07'/>\n    </vsock>\n",
		"empty":     "    <vsock model='virtio'/>\n",
	} {
		got, ok := dropVsockDevice([]byte(head + device + tail))
		assert.True(t, ok, name)
		assert.Equal(t, head+tail, string(got), name)
	}
	got, ok := dropVsockDevice([]byte(head + tail))
	assert.False(t, ok)
	assert.Equal(t, head+tail, string(got))
}

// A hypervisor with no vsock support is not asked for a device it can not
// give a guest.
func TestVirtInstWithVsockOnAHostWithNoVsock(t *testing.T) {
	saved := hostVsockDevice
	hostVsockDevice = filepath.Join(t.TempDir(), "none")
	defer func() { hostVsockDevice = saved }()
	assert.False(t, hasHostVsock())
	r := &T{VsockPort: 1215, VirtInst: []string{"virt-install", "--name", "vm1"}}
	r.SetLoggerForTest(plog.NewDefaultLogger())
	assert.Equal(t, r.VirtInst, r.virtInstWithVsock(context.Background()))
}

func TestVsockCID(t *testing.T) {
	for s, want := range map[string]int{
		`<vsock model='virtio'><cid auto='yes' address='3'/></vsock>`: 3,
		`<vsock model="virtio"><cid auto="no" address="42"/></vsock>`: 42,
		`<cid address='7' auto='no'/>`:                                7,
	} {
		got, ok := vsockCID([]byte(s))
		assert.True(t, ok, s)
		assert.Equal(t, want, got, s)
	}
	for _, s := range []string{
		`<domain></domain>`,
		`<vsock model='virtio'><cid auto='yes'/></vsock>`,
	} {
		_, ok := vsockCID([]byte(s))
		assert.False(t, ok, s)
	}
}

// A virsh create of the domain leaves its definition, and what the nodes sync.
func TestDefinitionWithoutVsock(t *testing.T) {
	saved := configDir
	configDir = t.TempDir()
	defer func() { configDir = saved }()
	r := &T{Name: "vm1"}

	path, err := r.definitionWithoutVsock()
	assert.Error(t, err, "no configuration file")
	assert.Empty(t, path)

	withDevice := "<domain>\n  <vsock model='virtio'><cid auto='yes'/></vsock>\n</domain>\n"
	require.NoError(t, os.WriteFile(r.configFile(), []byte(withDevice), 0o644))
	path, err = r.definitionWithoutVsock()
	require.NoError(t, err)
	defer func() { _ = os.Remove(path) }()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "<domain>\n</domain>\n", string(b))
	b, err = os.ReadFile(r.configFile())
	require.NoError(t, err)
	assert.Equal(t, withDevice, string(b), "the definition is left as it is")

	require.NoError(t, os.WriteFile(r.configFile(), []byte("<domain>\n</domain>\n"), 0o644))
	path, err = r.definitionWithoutVsock()
	require.NoError(t, err)
	assert.Empty(t, path, "no device to drop")
}
