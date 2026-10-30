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
		assert.Equal(t, r.VirtInst, r.virtInstWithVsock(context.Background(), r.VirtInst), name)
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
	assert.Equal(t, r.VirtInst, r.virtInstWithVsock(context.Background(), r.VirtInst))
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

func TestVMSpecArgs(t *testing.T) {
	s := vmSpec{
		name:     "vm1",
		vcpus:    2,
		memoryMB: 2048,
		osFlag:   "--osinfo",
		osValue:  "ubuntu24.04",
		disks:    []string{"/dev/zvol/tank/vm1", "/dev/sdb"},
		bridges:  []string{"br-prd"},
		userData: "/tmp/ud",
		extra:    []string{"--boot", "uefi"},
	}
	assert.Equal(t, []string{
		"virt-install", "--name", "vm1", "--memory", "2048", "--vcpus", "2", "--osinfo", "ubuntu24.04", "--import",
		"--disk", "/dev/zvol/tank/vm1,format=raw,cache=none", "--disk", "/dev/sdb,format=raw,cache=none",
		"--network", "bridge=br-prd,model=virtio",
		"--cloud-init", "user-data=/tmp/ud",
		"--noautoconsole",
		"--boot", "uefi",
	}, s.args())
}

// A virt-install older than the --osinfo name takes --os-variant, and a guest
// naming no os does not stop a virt-install requiring one.
func TestOSOption(t *testing.T) {
	for _, tc := range []struct {
		name, help, osinfo, flag, value string
	}{
		{"recent, detected", "  --osinfo OSINFO", "", "--osinfo", "detect=on,require=off"},
		{"recent, named", "  --osinfo OSINFO", "rhel9.0", "--osinfo", "rhel9.0"},
		{"older, generic", "  --os-variant", "", "--os-variant", "generic"},
		{"older, named", "  --os-variant", "rhel9.0", "--os-variant", "rhel9.0"},
	} {
		flag, value := osOption([]byte(tc.help), tc.osinfo)
		assert.Equal(t, tc.flag, flag, tc.name)
		assert.Equal(t, tc.value, value, tc.name)
	}
}

func TestSpecError(t *testing.T) {
	size := int64(1 << 30)
	assert.NoError(t, (&T{}).specError(), "a domain defined by a sysadmin")
	assert.NoError(t, (&T{VirtInst: []string{"virt-install"}}).specError(), "virtinst alone")
	assert.NoError(t, (&T{VCPUs: 2, Memory: &size, Disk: []string{"disk#1"}}).specError())
	assert.Error(t, (&T{VCPUs: 2, Memory: &size, Disk: []string{"disk#1"}, VirtInst: []string{"x"}}).specError(), "virtinst says all of it")
	assert.ErrorContains(t, (&T{VCPUs: 2}).specError(), "memory, disk")
	assert.ErrorContains(t, (&T{Image: "/a"}).specError(), "vcpus, memory, disk")
	small := int64(512)
	assert.ErrorContains(t, (&T{VCPUs: 2, Memory: &small, Disk: []string{"disk#1"}}).specError(), "memory")
}

func TestParseImageInfo(t *testing.T) {
	format, err := parseImageInfo([]byte(`{"format": "qcow2", "virtual-size": 1024}`))
	assert.NoError(t, err)
	assert.Equal(t, "qcow2", format)
	_, err = parseImageInfo([]byte(`{"format": "qcow2", "backing-filename": "/etc/shadow"}`))
	assert.ErrorContains(t, err, "backing file")
	_, err = parseImageInfo([]byte(`{"format": "qcow2", "format-specific": {"data": {"data-file": "/dev/sda"}}}`))
	assert.ErrorContains(t, err, "data file")
	_, err = parseImageInfo([]byte(`{"format": "vmdk"}`))
	assert.ErrorContains(t, err, "qcow2 or raw")
}

// A follower runs no command of the keywords: it would write the image over a
// disk it may share with the leader, and start a second guest on it.
func TestProvisionAsFollowerLeavesAGuestToTheLeader(t *testing.T) {
	r := &T{Image: "/does/not/exist", VCPUs: 1}
	r.SetLoggerForTest(plog.NewDefaultLogger())
	assert.NoError(t, r.ProvisionAsFollower(context.Background()))
}
